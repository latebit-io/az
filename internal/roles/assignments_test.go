package roles

import (
	"context"
	"testing"

	"github.com/latebit-io/az/internal/resources"
	"github.com/latebit-io/az/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAssignmentFixture(t *testing.T) (AssignmentService, RoleService, context.Context) {
	pool := utils.NewTestPool(t)
	resourceService := resources.NewDefaultResourceService(resources.NewPostgresResourceTypeRepository(pool), utils.NewPostgresTxManager(pool))
	roleService := NewDefaultRoleService(NewPostgresRoleRepository(pool), utils.NewPostgresTxManager(pool))
	assignmentService := NewDefaultAssignmentService(NewPostgresAssignmentRepository(pool))
	ctx := context.Background()
	require.NoError(t, resourceService.Create(ctx, "default", "document", []string{"read"}))
	require.NoError(t, roleService.Create(ctx, "default", "viewer", "Viewer", "",
		[]Permission{{Resource: "document", Action: "read"}}))
	require.NoError(t, roleService.Create(ctx, "default", "admin", "Admin", "", nil))
	return assignmentService, roleService, ctx
}

func TestAssignmentService_AssignAndRolesForSubject(t *testing.T) {
	service, _, ctx := newAssignmentFixture(t)

	require.NoError(t, service.Assign(ctx, "default", "user@example.com", "viewer"))
	require.NoError(t, service.Assign(ctx, "default", "user@example.com", "admin"))

	roleKeys, err := service.RolesForSubject(ctx, "default", "user@example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"admin", "viewer"}, roleKeys)

	roleKeys, err = service.RolesForSubject(ctx, "default", "nobody")
	require.NoError(t, err)
	assert.Empty(t, roleKeys)
}

func TestAssignmentService_UnknownRole(t *testing.T) {
	service, _, ctx := newAssignmentFixture(t)

	err := service.Assign(ctx, "default", "user@example.com", "missing")
	var notFound RoleNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestAssignmentService_Duplicate(t *testing.T) {
	service, _, ctx := newAssignmentFixture(t)

	require.NoError(t, service.Assign(ctx, "default", "user@example.com", "viewer"))
	err := service.Assign(ctx, "default", "user@example.com", "viewer")
	var duplicate AssignmentDuplicateError
	assert.ErrorAs(t, err, &duplicate)
}

func TestAssignmentService_ListFilters(t *testing.T) {
	service, _, ctx := newAssignmentFixture(t)

	require.NoError(t, service.Assign(ctx, "default", "user-1", "viewer"))
	require.NoError(t, service.Assign(ctx, "default", "user-1", "admin"))
	require.NoError(t, service.Assign(ctx, "default", "user-2", "viewer"))

	all, err := service.List(ctx, "default", "", "")
	require.NoError(t, err)
	assert.Len(t, all, 3)

	bySubject, err := service.List(ctx, "default", "user-1", "")
	require.NoError(t, err)
	assert.Len(t, bySubject, 2)

	byRole, err := service.List(ctx, "default", "", "viewer")
	require.NoError(t, err)
	assert.Len(t, byRole, 2)

	both, err := service.List(ctx, "default", "user-2", "viewer")
	require.NoError(t, err)
	assert.Len(t, both, 1)
}

func TestAssignmentService_Unassign(t *testing.T) {
	service, _, ctx := newAssignmentFixture(t)

	require.NoError(t, service.Assign(ctx, "default", "user-1", "viewer"))
	require.NoError(t, service.Unassign(ctx, "default", "user-1", "viewer"))

	err := service.Unassign(ctx, "default", "user-1", "viewer")
	var notFound AssignmentNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestAssignmentService_CascadeOnRoleDelete(t *testing.T) {
	service, roleService, ctx := newAssignmentFixture(t)

	require.NoError(t, service.Assign(ctx, "default", "user-1", "viewer"))
	require.NoError(t, roleService.Delete(ctx, "default", "viewer"))

	roleKeys, err := service.RolesForSubject(ctx, "default", "user-1")
	require.NoError(t, err)
	assert.Empty(t, roleKeys)
}
