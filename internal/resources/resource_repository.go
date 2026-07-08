package resources

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/latebit-io/az/internal/utils"
)

const pgUniqueViolation = "23505"

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

func (r *PostgresResourceTypeRepository) Create(ctx context.Context, resourceType ResourceType) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	attributes, err := marshalAttributes(resourceType.Attributes)
	if err != nil {
		return err
	}
	_, err = querier.Exec(ctx,
		`INSERT INTO resource_types (tenant_id, key, name, description, actions, attributes)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		resourceType.TenantID, resourceType.Key, resourceType.Name, resourceType.Description,
		resourceType.Actions, attributes)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return ResourceTypeDuplicateError{Value: resourceType.Key}
	}
	return err
}

func (r *PostgresResourceTypeRepository) Read(ctx context.Context, tenantID, key string) (*ResourceType, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	row := querier.QueryRow(ctx,
		`SELECT id, tenant_id, key, name, description, actions, attributes, created, modified
		 FROM resource_types WHERE tenant_id = $1 AND key = $2`, tenantID, key)
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
		`SELECT id, tenant_id, key, name, description, actions, attributes, created, modified
		 FROM resource_types WHERE tenant_id = $1 ORDER BY key`, tenantID)
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

func (r *PostgresResourceTypeRepository) Update(ctx context.Context, resourceType ResourceType) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	attributes, err := marshalAttributes(resourceType.Attributes)
	if err != nil {
		return err
	}
	tag, err := querier.Exec(ctx,
		`UPDATE resource_types SET name = $3, description = $4, actions = $5, attributes = $6, modified = now()
		 WHERE tenant_id = $1 AND key = $2`,
		resourceType.TenantID, resourceType.Key, resourceType.Name, resourceType.Description,
		resourceType.Actions, attributes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ResourceTypeNotFoundError{Value: resourceType.Key}
	}
	return nil
}

func (r *PostgresResourceTypeRepository) Delete(ctx context.Context, tenantID, key string) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx, "DELETE FROM resource_types WHERE tenant_id = $1 AND key = $2", tenantID, key)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ResourceTypeNotFoundError{Value: key}
	}
	return nil
}

func marshalAttributes(attributes []AttributeDef) ([]byte, error) {
	if attributes == nil {
		attributes = []AttributeDef{}
	}
	return json.Marshal(attributes)
}

func scanResourceType(row pgx.Row) (*ResourceType, error) {
	var resourceType ResourceType
	var attributes []byte
	err := row.Scan(&resourceType.ID, &resourceType.TenantID, &resourceType.Key, &resourceType.Name,
		&resourceType.Description, &resourceType.Actions, &attributes, &resourceType.Created, &resourceType.Modified)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(attributes, &resourceType.Attributes); err != nil {
		return nil, err
	}
	return &resourceType, nil
}
