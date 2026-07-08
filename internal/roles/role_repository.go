package roles

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

type RoleRepository interface {
	Create(ctx context.Context, role Role) error
	Read(ctx context.Context, tenantID, key string) (*Role, error)
	ReadAll(ctx context.Context, tenantID string) ([]Role, error)
	Update(ctx context.Context, role Role) error
	Delete(ctx context.Context, tenantID, key string) error
	// AnyGrants reports whether any of the given roles grants resource:action,
	// using jsonb containment against the role's permissions (GIN indexed).
	AnyGrants(ctx context.Context, tenantID string, roleKeys []string, resource, action string) (string, bool, error)
	// AnyReferencesResource reports whether any role grants a permission on
	// the resource type.
	AnyReferencesResource(ctx context.Context, tenantID, resource string) (bool, error)
}

type PostgresRoleRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRoleRepository(pool *pgxpool.Pool) RoleRepository {
	return &PostgresRoleRepository{pool: pool}
}

func (r *PostgresRoleRepository) Create(ctx context.Context, role Role) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	permissions, err := marshalPermissions(role.Permissions)
	if err != nil {
		return err
	}
	_, err = querier.Exec(ctx,
		"INSERT INTO roles (tenant_id, key, name, description, permissions) VALUES ($1, $2, $3, $4, $5)",
		role.TenantID, role.Key, role.Name, role.Description, permissions)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return RoleDuplicateError{Value: role.Key}
	}
	return err
}

func (r *PostgresRoleRepository) Read(ctx context.Context, tenantID, key string) (*Role, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	row := querier.QueryRow(ctx,
		`SELECT id, tenant_id, key, name, description, permissions, created, modified
		 FROM roles WHERE tenant_id = $1 AND key = $2`, tenantID, key)
	role, err := scanRole(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, RoleNotFoundError{Value: key}
	}
	if err != nil {
		return nil, err
	}
	return role, nil
}

func (r *PostgresRoleRepository) ReadAll(ctx context.Context, tenantID string) ([]Role, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	rows, err := querier.Query(ctx,
		`SELECT id, tenant_id, key, name, description, permissions, created, modified
		 FROM roles WHERE tenant_id = $1 ORDER BY key`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roleList []Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return roleList, err
		}
		roleList = append(roleList, *role)
	}
	return roleList, rows.Err()
}

func (r *PostgresRoleRepository) Update(ctx context.Context, role Role) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	permissions, err := marshalPermissions(role.Permissions)
	if err != nil {
		return err
	}
	tag, err := querier.Exec(ctx,
		`UPDATE roles SET name = $3, description = $4, permissions = $5, modified = now()
		 WHERE tenant_id = $1 AND key = $2`,
		role.TenantID, role.Key, role.Name, role.Description, permissions)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return RoleNotFoundError{Value: role.Key}
	}
	return nil
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
	grant, err := json.Marshal([]Permission{{Resource: resource, Action: action}})
	if err != nil {
		return "", false, err
	}
	var roleKey string
	err = querier.QueryRow(ctx,
		"SELECT key FROM roles WHERE tenant_id = $1 AND key = ANY($2) AND permissions @> $3 LIMIT 1",
		tenantID, roleKeys, grant).Scan(&roleKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return roleKey, true, nil
}

func (r *PostgresRoleRepository) AnyReferencesResource(ctx context.Context, tenantID, resource string) (bool, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	grant, err := json.Marshal([]map[string]string{{"resource": resource}})
	if err != nil {
		return false, err
	}
	var referenced bool
	err = querier.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM roles WHERE tenant_id = $1 AND permissions @> $2)",
		tenantID, grant).Scan(&referenced)
	return referenced, err
}

func marshalPermissions(permissions []Permission) ([]byte, error) {
	if permissions == nil {
		permissions = []Permission{}
	}
	return json.Marshal(permissions)
}

func scanRole(row pgx.Row) (*Role, error) {
	var role Role
	var permissions []byte
	err := row.Scan(&role.ID, &role.TenantID, &role.Key, &role.Name, &role.Description, &permissions,
		&role.Created, &role.Modified)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(permissions, &role.Permissions); err != nil {
		return nil, err
	}
	return &role, nil
}
