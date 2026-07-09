package roles

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/latebit-io/az/internal/utils"
)

const pgForeignKeyViolation = "23503"

type AssignmentRepository interface {
	Create(ctx context.Context, assignment RoleAssignment) error
	ReadAll(ctx context.Context, tenantID, subject, role string) ([]RoleAssignment, error)
	Delete(ctx context.Context, tenantID, subject, role string) error
	RolesForSubject(ctx context.Context, tenantID, subject string) ([]string, error)
}

type PostgresAssignmentRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresAssignmentRepository(pool *pgxpool.Pool) AssignmentRepository {
	return &PostgresAssignmentRepository{pool: pool}
}

func (r *PostgresAssignmentRepository) Create(ctx context.Context, assignment RoleAssignment) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	_, err := querier.Exec(ctx, "INSERT INTO role_assignments (tenant_id, subject, role) VALUES ($1, $2, $3)",
		assignment.TenantID, assignment.Subject, assignment.Role)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return AssignmentDuplicateError{Subject: assignment.Subject, Role: assignment.Role}
		case pgForeignKeyViolation:
			return RoleNotFoundError{Value: assignment.Role}
		}
	}
	return err
}

func (r *PostgresAssignmentRepository) ReadAll(ctx context.Context, tenantID, subject, role string) ([]RoleAssignment, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	rows, err := querier.Query(ctx,
		`SELECT id, tenant_id, subject, role, created FROM role_assignments
		 WHERE tenant_id = $1 AND ($2 = '' OR subject = $2) AND ($3 = '' OR role = $3)
		 ORDER BY subject, role`, tenantID, subject, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []RoleAssignment
	for rows.Next() {
		var assignment RoleAssignment
		if err := rows.Scan(&assignment.ID, &assignment.TenantID, &assignment.Subject, &assignment.Role,
			&assignment.Created); err != nil {
			return assignments, err
		}
		assignments = append(assignments, assignment)
	}
	return assignments, rows.Err()
}

func (r *PostgresAssignmentRepository) Delete(ctx context.Context, tenantID, subject, role string) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx,
		"DELETE FROM role_assignments WHERE tenant_id = $1 AND subject = $2 AND role = $3", tenantID, subject, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return AssignmentNotFoundError{Subject: subject, Role: role}
	}
	return nil
}

func (r *PostgresAssignmentRepository) RolesForSubject(ctx context.Context, tenantID, subject string) ([]string, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	rows, err := querier.Query(ctx,
		"SELECT role FROM role_assignments WHERE tenant_id = $1 AND subject = $2 ORDER BY role", tenantID, subject)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roleKeys []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return roleKeys, err
		}
		roleKeys = append(roleKeys, role)
	}
	return roleKeys, rows.Err()
}
