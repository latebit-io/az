package conditions

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/latebit-io/az/internal/utils"
)

type RuleRepository interface {
	Create(ctx context.Context, rule ConditionSetRule) error
	ReadAll(ctx context.Context, tenantID string) ([]ConditionSetRule, error)
	// ReadForPermission returns the rules granting resource:action — the check
	// path hot query.
	ReadForPermission(ctx context.Context, tenantID, resource, action string) ([]ConditionSetRule, error)
	Delete(ctx context.Context, rule ConditionSetRule) error
	// AnyReferencesResource reports whether any rule references the resource
	// type.
	AnyReferencesResource(ctx context.Context, tenantID, resource string) (bool, error)
}

type PostgresRuleRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRuleRepository(pool *pgxpool.Pool) RuleRepository {
	return &PostgresRuleRepository{pool: pool}
}

func (r *PostgresRuleRepository) Create(ctx context.Context, rule ConditionSetRule) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	_, err := querier.Exec(ctx,
		`INSERT INTO condition_set_rules (tenant_id, subject_set, resource, action, resource_set)
		 VALUES ($1, $2, $3, $4, $5)`,
		rule.TenantID, rule.SubjectSet, rule.Resource, rule.Action, rule.ResourceSet)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return RuleDuplicateError{Value: rule.String()}
	}
	return err
}

func (r *PostgresRuleRepository) ReadAll(ctx context.Context, tenantID string) ([]ConditionSetRule, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	rows, err := querier.Query(ctx,
		`SELECT id, tenant_id, subject_set, resource, action, resource_set, created
		 FROM condition_set_rules WHERE tenant_id = $1 ORDER BY resource, action, subject_set`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []ConditionSetRule
	for rows.Next() {
		var rule ConditionSetRule
		if err := rows.Scan(&rule.ID, &rule.TenantID, &rule.SubjectSet, &rule.Resource, &rule.Action,
			&rule.ResourceSet, &rule.Created); err != nil {
			return rules, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *PostgresRuleRepository) ReadForPermission(ctx context.Context, tenantID, resource,
	action string) ([]ConditionSetRule, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	rows, err := querier.Query(ctx,
		`SELECT id, tenant_id, subject_set, resource, action, resource_set, created
		 FROM condition_set_rules WHERE tenant_id = $1 AND resource = $2 AND action = $3
		 ORDER BY subject_set, resource_set`, tenantID, resource, action)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []ConditionSetRule
	for rows.Next() {
		var rule ConditionSetRule
		if err := rows.Scan(&rule.ID, &rule.TenantID, &rule.SubjectSet, &rule.Resource, &rule.Action,
			&rule.ResourceSet, &rule.Created); err != nil {
			return rules, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *PostgresRuleRepository) Delete(ctx context.Context, rule ConditionSetRule) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx,
		`DELETE FROM condition_set_rules
		 WHERE tenant_id = $1 AND subject_set = $2 AND resource = $3 AND action = $4 AND resource_set = $5`,
		rule.TenantID, rule.SubjectSet, rule.Resource, rule.Action, rule.ResourceSet)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return RuleNotFoundError{Value: rule.String()}
	}
	return nil
}

func (r *PostgresRuleRepository) AnyReferencesResource(ctx context.Context, tenantID, resource string) (bool, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	var referenced bool
	err := querier.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM condition_set_rules WHERE tenant_id = $1 AND resource = $2)",
		tenantID, resource).Scan(&referenced)
	return referenced, err
}
