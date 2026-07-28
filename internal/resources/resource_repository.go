package resources

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/latebit-io/az/internal/utils"
)

const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

type ResourceTypeRepository interface {
	Create(ctx context.Context, resourceType ResourceType) (*ResourceType, error)
	Read(ctx context.Context, tenantID, id string) (*ResourceType, error)
	ReadAll(ctx context.Context, tenantID string) ([]ResourceType, error)
	Update(ctx context.Context, resourceType ResourceType) error
	Delete(ctx context.Context, tenantID, id string) error
}

type PostgresResourceTypeRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresResourceTypeRepository(pool *pgxpool.Pool) ResourceTypeRepository {
	return &PostgresResourceTypeRepository{pool: pool}
}

const selectResourceType = `
	SELECT rt.id, rt.tenant_id, rt.name, rt.created, rt.modified,
	       COALESCE(array_agg(a.action ORDER BY a.action) FILTER (WHERE a.action IS NOT NULL), '{}')
	FROM resource_types rt
	LEFT JOIN resource_type_actions a ON a.resource_type_id = rt.id`

func (r *PostgresResourceTypeRepository) Create(ctx context.Context, resourceType ResourceType) (*ResourceType, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	// time-ordered uuids (v7) keep index inserts local instead of scattering
	// across the btree like random v4 ids
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	resourceType.ID = id.String()
	err = querier.QueryRow(ctx,
		"INSERT INTO resource_types (id, tenant_id, name) VALUES ($1, $2, $3) RETURNING created, modified",
		resourceType.ID, resourceType.TenantID, resourceType.Name).Scan(&resourceType.Created,
		&resourceType.Modified)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return nil, ResourceTypeDuplicateError{Value: resourceType.Name}
	}
	if err != nil {
		return nil, err
	}
	if err := insertActions(ctx, querier, resourceType); err != nil {
		return nil, err
	}
	return &resourceType, nil
}

func (r *PostgresResourceTypeRepository) Read(ctx context.Context, tenantID, id string) (*ResourceType, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	row := querier.QueryRow(ctx,
		selectResourceType+" WHERE rt.tenant_id = $1 AND rt.id = $2 GROUP BY rt.id", tenantID, id)
	resourceType, err := scanResourceType(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ResourceTypeNotFoundError{Value: id}
	}
	if err != nil {
		return nil, err
	}
	return resourceType, nil
}

func (r *PostgresResourceTypeRepository) ReadAll(ctx context.Context, tenantID string) ([]ResourceType, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	rows, err := querier.Query(ctx,
		selectResourceType+" WHERE rt.tenant_id = $1 GROUP BY rt.id ORDER BY rt.name", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resourceTypes := []ResourceType{}
	for rows.Next() {
		resourceType, err := scanResourceType(rows)
		if err != nil {
			return resourceTypes, err
		}
		resourceTypes = append(resourceTypes, *resourceType)
	}
	return resourceTypes, rows.Err()
}

// Update replaces the name and action set: removed actions are deleted
// (refused by FK RESTRICT while a role permission references them), new ones
// inserted.
func (r *PostgresResourceTypeRepository) Update(ctx context.Context, resourceType ResourceType) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx,
		"UPDATE resource_types SET name = $3, modified = now() WHERE tenant_id = $1 AND id = $2",
		resourceType.TenantID, resourceType.ID, resourceType.Name)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return ResourceTypeDuplicateError{Value: resourceType.Name}
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ResourceTypeNotFoundError{Value: resourceType.ID}
	}

	_, err = querier.Exec(ctx,
		"DELETE FROM resource_type_actions WHERE resource_type_id = $1 AND action != ALL($2)",
		resourceType.ID, resourceType.Actions)
	if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
		return ResourceTypeReferencedError{Value: resourceType.Name}
	}
	if err != nil {
		return err
	}
	return insertActions(ctx, querier, resourceType)
}

func (r *PostgresResourceTypeRepository) Delete(ctx context.Context, tenantID, id string) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx, "DELETE FROM resource_types WHERE tenant_id = $1 AND id = $2", tenantID, id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
		return ResourceTypeReferencedError{Value: id}
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ResourceTypeNotFoundError{Value: id}
	}
	return nil
}

func insertActions(ctx context.Context, querier utils.Querier, resourceType ResourceType) error {
	for _, action := range resourceType.Actions {
		_, err := querier.Exec(ctx,
			`INSERT INTO resource_type_actions (resource_type_id, action) VALUES ($1, $2)
			 ON CONFLICT DO NOTHING`,
			resourceType.ID, action)
		if err != nil {
			return err
		}
	}
	return nil
}

func scanResourceType(row pgx.Row) (*ResourceType, error) {
	var resourceType ResourceType
	err := row.Scan(&resourceType.ID, &resourceType.TenantID, &resourceType.Name, &resourceType.Created,
		&resourceType.Modified, &resourceType.Actions)
	if err != nil {
		return nil, err
	}
	return &resourceType, nil
}
