package check

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/latebit-io/az/internal/utils"
)

// Grant is the policy state a single check resolves to: whether the resource
// type exists, whether it declares the action, and which assigned role — if
// any — grants it to the subject. Every field is a decision input, never an
// error: an unknown type or action is a deny.
type Grant struct {
	TypeFound      bool
	ActionDeclared bool
	Granted        bool
	RoleName       string
}

type CheckRepository interface {
	// Resolve answers a check in one round trip. The resource type is
	// resolved by tenant-scoped name, the action is matched against its
	// declared actions, and a granting role is found by lateral join —
	// no service-side lookups between statements.
	Resolve(ctx context.Context, tenantID string, request CheckRequest) (Grant, error)
}

type PostgresCheckRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresCheckRepository(pool *pgxpool.Pool) CheckRepository {
	return &PostgresCheckRepository{pool: pool}
}

// resolveGrant answers the whole check. The row exists iff the resource type
// does; the action column is null unless declared; the lateral subquery stops
// at the first granting role. role_permissions has a foreign key onto
// resource_type_actions, so an undeclared action can never join a grant.
const resolveGrant = `
	SELECT a.action IS NOT NULL, g.name
	FROM resource_types rt
	LEFT JOIN resource_type_actions a
	       ON a.resource_type_id = rt.id AND a.action = $4
	LEFT JOIN LATERAL (
	    SELECT r.name
	    FROM role_assignments ra
	    JOIN role_permissions p
	      ON p.role_id = ra.role_id AND p.resource_type_id = rt.id AND p.action = $4
	    JOIN roles r ON r.id = ra.role_id AND r.tenant_id = ra.tenant_id
	    WHERE ra.tenant_id = $1 AND ra.subject = $3
	    LIMIT 1
	) g ON true
	WHERE rt.tenant_id = $1 AND rt.name = $2`

func (r *PostgresCheckRepository) Resolve(ctx context.Context, tenantID string,
	request CheckRequest) (Grant, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	var declared bool
	var roleName *string
	err := querier.QueryRow(ctx, resolveGrant, tenantID, request.Resource, request.Subject,
		request.Action).Scan(&declared, &roleName)
	if errors.Is(err, pgx.ErrNoRows) {
		return Grant{}, nil
	}
	if err != nil {
		return Grant{}, err
	}
	grant := Grant{TypeFound: true, ActionDeclared: declared}
	if roleName != nil {
		grant.Granted = true
		grant.RoleName = *roleName
	}
	return grant, nil
}
