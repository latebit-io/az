package roles

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/latebit-io/az/internal/utils"
)

const pgUniqueViolation = "23505"

type RoleRepository interface {
	Create(ctx context.Context, role Role) error
	Read(ctx context.Context, tenantID, key string) (*Role, error)
	ReadAll(ctx context.Context, tenantID string) ([]Role, error)
	Update(ctx context.Context, role Role) error
	Delete(ctx context.Context, tenantID, key string) error
	// AnyGrants reports whether any of the given roles grants resource:action.
	AnyGrants(ctx context.Context, tenantID string, roleKeys []string, resource, action string) (string, bool, error)
}

type PostgresRoleRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRoleRepository(pool *pgxpool.Pool) RoleRepository {
	return &PostgresRoleRepository{pool: pool}
}

func (r *PostgresRoleRepository) Create(ctx context.Context, role Role) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	_, err := querier.Exec(ctx,
		"INSERT INTO roles (tenant_id, key, name, description) VALUES ($1, $2, $3, $4)",
		role.TenantID, role.Key, role.Name, role.Description)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return RoleDuplicateError{Value: role.Key}
	}
	if err != nil {
		return err
	}
	return insertPermissions(ctx, querier, role)
}

func (r *PostgresRoleRepository) Read(ctx context.Context, tenantID, key string) (*Role, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	var role Role
	err := querier.QueryRow(ctx,
		"SELECT id, tenant_id, key, name, description, created, modified FROM roles WHERE tenant_id = $1 AND key = $2",
		tenantID, key).Scan(&role.ID, &role.TenantID, &role.Key, &role.Name, &role.Description,
		&role.Created, &role.Modified)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, RoleNotFoundError{Value: key}
	}
	if err != nil {
		return nil, err
	}

	rows, err := querier.Query(ctx,
		`SELECT resource, action FROM role_permissions WHERE tenant_id = $1 AND role = $2
		 ORDER BY resource, action`, tenantID, key)
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
		"SELECT id, tenant_id, key, name, description, created, modified FROM roles WHERE tenant_id = $1 ORDER BY key",
		tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roleList []Role
	indexByKey := map[string]int{}
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.TenantID, &role.Key, &role.Name, &role.Description,
			&role.Created, &role.Modified); err != nil {
			return roleList, err
		}
		indexByKey[role.Key] = len(roleList)
		roleList = append(roleList, role)
	}
	if err := rows.Err(); err != nil {
		return roleList, err
	}

	permissionRows, err := querier.Query(ctx,
		"SELECT role, resource, action FROM role_permissions WHERE tenant_id = $1 ORDER BY resource, action",
		tenantID)
	if err != nil {
		return roleList, err
	}
	defer permissionRows.Close()
	for permissionRows.Next() {
		var roleKey string
		var permission Permission
		if err := permissionRows.Scan(&roleKey, &permission.Resource, &permission.Action); err != nil {
			return roleList, err
		}
		if index, ok := indexByKey[roleKey]; ok {
			roleList[index].Permissions = append(roleList[index].Permissions, permission)
		}
	}
	return roleList, permissionRows.Err()
}

// Update replaces the role's name, description and permission grants.
func (r *PostgresRoleRepository) Update(ctx context.Context, role Role) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx,
		"UPDATE roles SET name = $3, description = $4, modified = now() WHERE tenant_id = $1 AND key = $2",
		role.TenantID, role.Key, role.Name, role.Description)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return RoleNotFoundError{Value: role.Key}
	}

	_, err = querier.Exec(ctx, "DELETE FROM role_permissions WHERE tenant_id = $1 AND role = $2",
		role.TenantID, role.Key)
	if err != nil {
		return err
	}
	return insertPermissions(ctx, querier, role)
}

func (r *PostgresRoleRepository) Delete(ctx context.Context, tenantID, key string) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx, "DELETE FROM roles WHERE tenant_id = $1 AND key = $2", tenantID, key)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return RoleNotFoundError{Value: key}
	}
	return nil
}

func (r *PostgresRoleRepository) AnyGrants(ctx context.Context, tenantID string, roleKeys []string,
	resource, action string) (string, bool, error) {
	if len(roleKeys) == 0 {
		return "", false, nil
	}
	querier := utils.QuerierFrom(ctx, r.pool)
	var roleKey string
	err := querier.QueryRow(ctx,
		`SELECT role FROM role_permissions
		 WHERE tenant_id = $1 AND role = ANY($2) AND resource = $3 AND action = $4 LIMIT 1`,
		tenantID, roleKeys, resource, action).Scan(&roleKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return roleKey, true, nil
}

// insertPermissions writes the grants; a foreign key violation means the
// resource:action pair was never declared on a resource type.
func insertPermissions(ctx context.Context, querier utils.Querier, role Role) error {
	for _, permission := range role.Permissions {
		_, err := querier.Exec(ctx,
			`INSERT INTO role_permissions (tenant_id, role, resource, action) VALUES ($1, $2, $3, $4)
			 ON CONFLICT DO NOTHING`,
			role.TenantID, role.Key, permission.Resource, permission.Action)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return InvalidPermissionError{Resource: permission.Resource, Action: permission.Action,
				Reason: "not declared on any resource type"}
		}
		if err != nil {
			return err
		}
	}
	return nil
}
