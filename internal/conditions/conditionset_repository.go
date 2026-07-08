package conditions

import (
	"context"
	"encoding/json"
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

type ConditionSetRepository interface {
	Create(ctx context.Context, set ConditionSet) error
	Read(ctx context.Context, tenantID, key string) (*ConditionSet, error)
	ReadAll(ctx context.Context, tenantID, setType string) ([]ConditionSet, error)
	// ReadMany returns the sets with the given keys, keyed by set key.
	ReadMany(ctx context.Context, tenantID string, keys []string) (map[string]ConditionSet, error)
	Update(ctx context.Context, set ConditionSet) error
	Delete(ctx context.Context, tenantID, key string) error
	// AnyReferencesResource reports whether any resource set is bound to the
	// resource type.
	AnyReferencesResource(ctx context.Context, tenantID, resource string) (bool, error)
}

type PostgresConditionSetRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresConditionSetRepository(pool *pgxpool.Pool) ConditionSetRepository {
	return &PostgresConditionSetRepository{pool: pool}
}

func (r *PostgresConditionSetRepository) Create(ctx context.Context, set ConditionSet) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	conditionsJSON, err := json.Marshal(set.Conditions)
	if err != nil {
		return err
	}
	_, err = querier.Exec(ctx,
		`INSERT INTO condition_sets (tenant_id, key, name, description, type, resource_type, conditions)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		set.TenantID, set.Key, set.Name, set.Description, set.Type, set.ResourceType, conditionsJSON)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return ConditionSetDuplicateError{Value: set.Key}
	}
	return err
}

func (r *PostgresConditionSetRepository) Read(ctx context.Context, tenantID, key string) (*ConditionSet, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	row := querier.QueryRow(ctx,
		`SELECT id, tenant_id, key, name, description, type, resource_type, conditions, created, modified
		 FROM condition_sets WHERE tenant_id = $1 AND key = $2`, tenantID, key)
	set, err := scanConditionSet(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ConditionSetNotFoundError{Value: key}
	}
	if err != nil {
		return nil, err
	}
	return set, nil
}

func (r *PostgresConditionSetRepository) ReadAll(ctx context.Context, tenantID, setType string) ([]ConditionSet, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	rows, err := querier.Query(ctx,
		`SELECT id, tenant_id, key, name, description, type, resource_type, conditions, created, modified
		 FROM condition_sets WHERE tenant_id = $1 AND ($2 = '' OR type = $2) ORDER BY key`, tenantID, setType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSets(rows)
}

func (r *PostgresConditionSetRepository) ReadMany(ctx context.Context, tenantID string,
	keys []string) (map[string]ConditionSet, error) {
	if len(keys) == 0 {
		return map[string]ConditionSet{}, nil
	}
	querier := utils.QuerierFrom(ctx, r.pool)
	rows, err := querier.Query(ctx,
		`SELECT id, tenant_id, key, name, description, type, resource_type, conditions, created, modified
		 FROM condition_sets WHERE tenant_id = $1 AND key = ANY($2)`, tenantID, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sets, err := collectSets(rows)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]ConditionSet, len(sets))
	for _, set := range sets {
		byKey[set.Key] = set
	}
	return byKey, nil
}

func (r *PostgresConditionSetRepository) Update(ctx context.Context, set ConditionSet) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	conditionsJSON, err := json.Marshal(set.Conditions)
	if err != nil {
		return err
	}
	tag, err := querier.Exec(ctx,
		`UPDATE condition_sets SET name = $3, description = $4, type = $5, resource_type = $6, conditions = $7,
		 modified = now() WHERE tenant_id = $1 AND key = $2`,
		set.TenantID, set.Key, set.Name, set.Description, set.Type, set.ResourceType, conditionsJSON)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ConditionSetNotFoundError{Value: set.Key}
	}
	return nil
}

func (r *PostgresConditionSetRepository) Delete(ctx context.Context, tenantID, key string) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx, "DELETE FROM condition_sets WHERE tenant_id = $1 AND key = $2", tenantID, key)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
		return ConditionSetReferencedError{Value: key}
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ConditionSetNotFoundError{Value: key}
	}
	return nil
}

func (r *PostgresConditionSetRepository) AnyReferencesResource(ctx context.Context, tenantID,
	resource string) (bool, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	var referenced bool
	err := querier.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM condition_sets WHERE tenant_id = $1 AND resource_type = $2)",
		tenantID, resource).Scan(&referenced)
	return referenced, err
}

func collectSets(rows pgx.Rows) ([]ConditionSet, error) {
	var sets []ConditionSet
	for rows.Next() {
		set, err := scanConditionSet(rows)
		if err != nil {
			return sets, err
		}
		sets = append(sets, *set)
	}
	return sets, rows.Err()
}

func scanConditionSet(row pgx.Row) (*ConditionSet, error) {
	var set ConditionSet
	var conditionsJSON []byte
	err := row.Scan(&set.ID, &set.TenantID, &set.Key, &set.Name, &set.Description, &set.Type, &set.ResourceType,
		&conditionsJSON, &set.Created, &set.Modified)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(conditionsJSON, &set.Conditions); err != nil {
		return nil, err
	}
	return &set, nil
}
