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
	ReadAll(ctx context.Context, tenantID, subject, roleID string) ([]RoleAssignment, error)
	Delete(ctx context.Context, tenantID, subject, roleID string) error
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
	_, err := querier.Exec(ctx, "INSERT INTO role_assignments (tenant_id, subject, role_id) VALUES ($1, $2, $3)",
		assignment.TenantID, assignment.Subject, assignment.RoleID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return AssignmentDuplicateError{Subject: assignment.Subject, RoleID: assignment.RoleID}
		case pgForeignKeyViolation:
			return RoleNotFoundError{Value: assignment.RoleID}
		}
	}
	return err
}

func (r *PostgresAssignmentRepository) ReadAll(ctx context.Context, tenantID, subject,
	roleID string) ([]RoleAssignment, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	rows, err := querier.Query(ctx,
		`SELECT tenant_id, subject, role_id, created FROM role_assignments
		 WHERE tenant_id = $1 AND ($2 = '' OR subject = $2) AND ($3 = '' OR role_id::text = $3)
		 ORDER BY subject`, tenantID, subject, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []RoleAssignment
	for rows.Next() {
		var assignment RoleAssignment
		if err := rows.Scan(&assignment.TenantID, &assignment.Subject, &assignment.RoleID,
			&assignment.Created); err != nil {
			return assignments, err
		}
		assignments = append(assignments, assignment)
	}
	return assignments, rows.Err()
}

func (r *PostgresAssignmentRepository) Delete(ctx context.Context, tenantID, subject, roleID string) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx,
		"DELETE FROM role_assignments WHERE tenant_id = $1 AND subject = $2 AND role_id = $3",
		tenantID, subject, roleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return AssignmentNotFoundError{Subject: subject, RoleID: roleID}
	}
	return nil
}

func (r *PostgresAssignmentRepository) RolesForSubject(ctx context.Context, tenantID, subject string) ([]string, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	rows, err := querier.Query(ctx,
		`SELECT r.name FROM role_assignments a JOIN roles r ON r.id = a.role_id
		 WHERE a.tenant_id = $1 AND a.subject = $2 ORDER BY r.name`, tenantID, subject)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roleNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return roleNames, err
		}
		roleNames = append(roleNames, name)
	}
	return roleNames, rows.Err()
}
