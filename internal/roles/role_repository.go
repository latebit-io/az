package roles

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/latebit-io/az/internal/utils"
)

const pgUniqueViolation = "23505"

type RoleRepository interface {
	Create(ctx context.Context, role Role) (*Role, error)
	Read(ctx context.Context, tenantID, id string) (*Role, error)
	ReadAll(ctx context.Context, tenantID string) ([]Role, error)
	Update(ctx context.Context, role Role) error
	Delete(ctx context.Context, tenantID, id string) error
	// SubjectGrant reports whether any role assigned to the subject grants
	// the action on the resource type, returning the granting role's name —
	// the check hot path.
	SubjectGrant(ctx context.Context, tenantID, subject, resourceTypeID, action string) (string, bool, error)
}

type PostgresRoleRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRoleRepository(pool *pgxpool.Pool) RoleRepository {
	return &PostgresRoleRepository{pool: pool}
}

func (r *PostgresRoleRepository) Create(ctx context.Context, role Role) (*Role, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	// time-ordered uuids (v7) keep index inserts local instead of scattering
	// across the btree like random v4 ids
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	role.ID = id.String()
	err = querier.QueryRow(ctx,
		"INSERT INTO roles (id, tenant_id, name) VALUES ($1, $2, $3) RETURNING created, modified",
		role.ID, role.TenantID, role.Name).Scan(&role.Created, &role.Modified)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return nil, RoleDuplicateError{Value: role.Name}
	}
	if err != nil {
		return nil, err
	}
	if err := insertPermissions(ctx, querier, role); err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *PostgresRoleRepository) Read(ctx context.Context, tenantID, id string) (*Role, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	var role Role
	err := querier.QueryRow(ctx,
		"SELECT id, tenant_id, name, created, modified FROM roles WHERE tenant_id = $1 AND id = $2",
		tenantID, id).Scan(&role.ID, &role.TenantID, &role.Name, &role.Created, &role.Modified)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, RoleNotFoundError{Value: id}
	}
	if err != nil {
		return nil, err
	}

	rows, err := querier.Query(ctx,
		`SELECT rt.name, p.action FROM role_permissions p
		 JOIN resource_types rt ON rt.id = p.resource_type_id
		 WHERE p.role_id = $1 ORDER BY rt.name, p.action`, role.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var permission Permission
		if err := rows.Scan(&permission.Resource, &permission.Action); err != nil {
			return nil, err
		}
		role.Permissions = append(role.Permissions, permission)
	}
	return &role, rows.Err()
}

func (r *PostgresRoleRepository) ReadAll(ctx context.Context, tenantID string) ([]Role, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	rows, err := querier.Query(ctx,
		"SELECT id, tenant_id, name, created, modified FROM roles WHERE tenant_id = $1 ORDER BY name", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roleList []Role
	indexByID := map[string]int{}
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.TenantID, &role.Name, &role.Created, &role.Modified); err != nil {
			return roleList, err
		}
		indexByID[role.ID] = len(roleList)
		roleList = append(roleList, role)
	}
	if err := rows.Err(); err != nil {
		return roleList, err
	}

	permissionRows, err := querier.Query(ctx,
		`SELECT p.role_id, rt.name, p.action FROM role_permissions p
		 JOIN roles r ON r.id = p.role_id
		 JOIN resource_types rt ON rt.id = p.resource_type_id
		 WHERE r.tenant_id = $1 ORDER BY rt.name, p.action`, tenantID)
	if err != nil {
		return roleList, err
	}
	defer permissionRows.Close()
	for permissionRows.Next() {
		var roleID string
		var permission Permission
		if err := permissionRows.Scan(&roleID, &permission.Resource, &permission.Action); err != nil {
			return roleList, err
		}
		if index, ok := indexByID[roleID]; ok {
			roleList[index].Permissions = append(roleList[index].Permissions, permission)
		}
	}
	return roleList, permissionRows.Err()
}

// Update replaces the role's name and permission grants.
func (r *PostgresRoleRepository) Update(ctx context.Context, role Role) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx,
		"UPDATE roles SET name = $3, modified = now() WHERE tenant_id = $1 AND id = $2",
		role.TenantID, role.ID, role.Name)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return RoleDuplicateError{Value: role.Name}
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return RoleNotFoundError{Value: role.ID}
	}

	if _, err := querier.Exec(ctx, "DELETE FROM role_permissions WHERE role_id = $1", role.ID); err != nil {
		return err
	}
	return insertPermissions(ctx, querier, role)
}

func (r *PostgresRoleRepository) Delete(ctx context.Context, tenantID, id string) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx, "DELETE FROM roles WHERE tenant_id = $1 AND id = $2", tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return RoleNotFoundError{Value: id}
	}
	return nil
}

func (r *PostgresRoleRepository) SubjectGrant(ctx context.Context, tenantID, subject, resourceTypeID,
	action string) (string, bool, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	var roleName string
	err := querier.QueryRow(ctx,
		`SELECT r.name FROM role_assignments a
		 JOIN role_permissions p ON p.role_id = a.role_id
		 JOIN roles r ON r.id = a.role_id
		 WHERE a.tenant_id = $1 AND a.subject = $2 AND p.resource_type_id = $3 AND p.action = $4
		 LIMIT 1`, tenantID, subject, resourceTypeID, action).Scan(&roleName)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return roleName, true, nil
}

// insertPermissions writes the grants, resolving resource names to type ids.
// Zero affected rows means the resource type name is unknown; a foreign key
// violation means the action was never declared on it.
func insertPermissions(ctx context.Context, querier utils.Querier, role Role) error {
	seen := map[Permission]bool{}
	for _, permission := range role.Permissions {
		if seen[permission] {
			continue
		}
		seen[permission] = true
		tag, err := querier.Exec(ctx,
			`INSERT INTO role_permissions (role_id, resource_type_id, action)
			 SELECT $1, rt.id, $4 FROM resource_types rt WHERE rt.tenant_id = $2 AND rt.name = $3`,
			role.ID, role.TenantID, permission.Resource, permission.Action)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return InvalidPermissionError{Resource: permission.Resource, Action: permission.Action,
				Reason: "action not declared on the resource type"}
		}
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return InvalidPermissionError{Resource: permission.Resource, Action: permission.Action,
				Reason: "unknown resource type"}
		}
	}
	return nil
}
