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

const pgForeignKeyViolation = "23503"

type InstanceRepository interface {
	Create(ctx context.Context, instance ResourceInstance) error
	Read(ctx context.Context, tenantID, resourceType, key string) (*ResourceInstance, error)
	ReadAll(ctx context.Context, tenantID, resourceType string) ([]ResourceInstance, error)
	Update(ctx context.Context, instance ResourceInstance) error
	Delete(ctx context.Context, tenantID, resourceType, key string) error
}

type PostgresInstanceRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresInstanceRepository(pool *pgxpool.Pool) InstanceRepository {
	return &PostgresInstanceRepository{pool: pool}
}

func (r *PostgresInstanceRepository) Create(ctx context.Context, instance ResourceInstance) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	attributes, err := marshalInstanceAttributes(instance.Attributes)
	if err != nil {
		return err
	}
	_, err = querier.Exec(ctx,
		`INSERT INTO resource_instances (tenant_id, resource_type, key, attributes) VALUES ($1, $2, $3, $4)`,
		instance.TenantID, instance.ResourceType, instance.Key, attributes)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return InstanceDuplicateError{Value: instance.Key}
		case pgForeignKeyViolation:
			return ResourceTypeNotFoundError{Value: instance.ResourceType}
		}
	}
	return err
}

func (r *PostgresInstanceRepository) Read(ctx context.Context, tenantID, resourceType, key string) (*ResourceInstance, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	row := querier.QueryRow(ctx,
		`SELECT id, tenant_id, resource_type, key, attributes, created, modified
		 FROM resource_instances WHERE tenant_id = $1 AND resource_type = $2 AND key = $3`,
		tenantID, resourceType, key)
	instance, err := scanInstance(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, InstanceNotFoundError{Value: key}
	}
	if err != nil {
		return nil, err
	}
	return instance, nil
}

func (r *PostgresInstanceRepository) ReadAll(ctx context.Context, tenantID, resourceType string) ([]ResourceInstance, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	rows, err := querier.Query(ctx,
		`SELECT id, tenant_id, resource_type, key, attributes, created, modified
		 FROM resource_instances WHERE tenant_id = $1 AND resource_type = $2 ORDER BY key`, tenantID, resourceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []ResourceInstance
	for rows.Next() {
		instance, err := scanInstance(rows)
		if err != nil {
			return instances, err
		}
		instances = append(instances, *instance)
	}
	return instances, rows.Err()
}

func (r *PostgresInstanceRepository) Update(ctx context.Context, instance ResourceInstance) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	attributes, err := marshalInstanceAttributes(instance.Attributes)
	if err != nil {
		return err
	}
	tag, err := querier.Exec(ctx,
		`UPDATE resource_instances SET attributes = $4, modified = now()
		 WHERE tenant_id = $1 AND resource_type = $2 AND key = $3`,
		instance.TenantID, instance.ResourceType, instance.Key, attributes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return InstanceNotFoundError{Value: instance.Key}
	}
	return nil
}

func (r *PostgresInstanceRepository) Delete(ctx context.Context, tenantID, resourceType, key string) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx,
		"DELETE FROM resource_instances WHERE tenant_id = $1 AND resource_type = $2 AND key = $3",
		tenantID, resourceType, key)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return InstanceNotFoundError{Value: key}
	}
	return nil
}

func marshalInstanceAttributes(attributes map[string]any) ([]byte, error) {
	if attributes == nil {
		attributes = map[string]any{}
	}
	return json.Marshal(attributes)
}

func scanInstance(row pgx.Row) (*ResourceInstance, error) {
	var instance ResourceInstance
	var attributes []byte
	err := row.Scan(&instance.ID, &instance.TenantID, &instance.ResourceType, &instance.Key, &attributes,
		&instance.Created, &instance.Modified)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(attributes, &instance.Attributes); err != nil {
		return nil, err
	}
	return &instance, nil
}
