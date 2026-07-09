package resources

import (
	"context"
	"errors"

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
	Create(ctx context.Context, resourceType ResourceType) error
	Read(ctx context.Context, tenantID, key string) (*ResourceType, error)
	ReadAll(ctx context.Context, tenantID string) ([]ResourceType, error)
	Update(ctx context.Context, resourceType ResourceType) error
	Delete(ctx context.Context, tenantID, key string) error
}

type PostgresResourceTypeRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresResourceTypeRepository(pool *pgxpool.Pool) ResourceTypeRepository {
	return &PostgresResourceTypeRepository{pool: pool}
}

const selectResourceType = `
	SELECT rt.id, rt.tenant_id, rt.key, rt.created, rt.modified,
	       COALESCE(array_agg(a.action ORDER BY a.action) FILTER (WHERE a.action IS NOT NULL), '{}')
	FROM resource_types rt
	LEFT JOIN resource_type_actions a ON a.tenant_id = rt.tenant_id AND a.resource = rt.key`

func (r *PostgresResourceTypeRepository) Create(ctx context.Context, resourceType ResourceType) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	_, err := querier.Exec(ctx, "INSERT INTO resource_types (tenant_id, key) VALUES ($1, $2)",
		resourceType.TenantID, resourceType.Key)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return ResourceTypeDuplicateError{Value: resourceType.Key}
	}
	if err != nil {
		return err
	}
	return insertActions(ctx, querier, resourceType)
}

func (r *PostgresResourceTypeRepository) Read(ctx context.Context, tenantID, key string) (*ResourceType, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	row := querier.QueryRow(ctx,
		selectResourceType+" WHERE rt.tenant_id = $1 AND rt.key = $2 GROUP BY rt.id", tenantID, key)
	resourceType, err := scanResourceType(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ResourceTypeNotFoundError{Value: key}
	}
	if err != nil {
		return nil, err
	}
	return resourceType, nil
}

func (r *PostgresResourceTypeRepository) ReadAll(ctx context.Context, tenantID string) ([]ResourceType, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	rows, err := querier.Query(ctx,
		selectResourceType+" WHERE rt.tenant_id = $1 GROUP BY rt.id ORDER BY rt.key", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var resourceTypes []ResourceType
	for rows.Next() {
		resourceType, err := scanResourceType(rows)
		if err != nil {
			return resourceTypes, err
		}
		resourceTypes = append(resourceTypes, *resourceType)
	}
	return resourceTypes, rows.Err()
}

// Update replaces the action set: removed actions are deleted (refused by FK
// RESTRICT while a role permission references them), new ones inserted.
func (r *PostgresResourceTypeRepository) Update(ctx context.Context, resourceType ResourceType) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx,
		"UPDATE resource_types SET modified = now() WHERE tenant_id = $1 AND key = $2",
		resourceType.TenantID, resourceType.Key)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ResourceTypeNotFoundError{Value: resourceType.Key}
	}

	_, err = querier.Exec(ctx,
		"DELETE FROM resource_type_actions WHERE tenant_id = $1 AND resource = $2 AND action != ALL($3)",
		resourceType.TenantID, resourceType.Key, resourceType.Actions)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
		return ResourceTypeReferencedError{Value: resourceType.Key}
	}
	if err != nil {
		return err
	}
	return insertActions(ctx, querier, resourceType)
}

func (r *PostgresResourceTypeRepository) Delete(ctx context.Context, tenantID, key string) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx, "DELETE FROM resource_types WHERE tenant_id = $1 AND key = $2", tenantID, key)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
		return ResourceTypeReferencedError{Value: key}
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ResourceTypeNotFoundError{Value: key}
	}
	return nil
}

func insertActions(ctx context.Context, querier utils.Querier, resourceType ResourceType) error {
	for _, action := range resourceType.Actions {
		_, err := querier.Exec(ctx,
			`INSERT INTO resource_type_actions (tenant_id, resource, action) VALUES ($1, $2, $3)
			 ON CONFLICT DO NOTHING`,
			resourceType.TenantID, resourceType.Key, action)
		if err != nil {
			return err
		}
	}
	return nil
}

func scanResourceType(row pgx.Row) (*ResourceType, error) {
	var resourceType ResourceType
	err := row.Scan(&resourceType.ID, &resourceType.TenantID, &resourceType.Key, &resourceType.Created,
		&resourceType.Modified, &resourceType.Actions)
	if err != nil {
		return nil, err
	}
	return &resourceType, nil
}
