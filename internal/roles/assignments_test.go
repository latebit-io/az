package roles

import (
	"context"
	"testing"

	"github.com/latebit-io/az/internal/resources"
	"github.com/latebit-io/az/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAssignmentFixture(t *testing.T) (AssignmentService, RoleService, *Role, *Role, context.Context) {
	pool := utils.NewTestPool(t)
	txManager := utils.NewPostgresTxManager(pool)
	resourceService := resources.NewDefaultResourceService(resources.NewPostgresResourceTypeRepository(pool),
		txManager)
	roleService := NewDefaultRoleService(NewPostgresRoleRepository(pool), txManager)
	assignmentService := NewDefaultAssignmentService(NewPostgresAssignmentRepository(pool))
	ctx := context.Background()
	{ _, err := resourceService.Create(ctx, "default", "document", []string{"read"}); require.NoError(t, err) }
	viewer, err := roleService.Create(ctx, "default", "Viewer", []Permission{{Resource: "document", Action: "read"}})
	require.NoError(t, err)
	admin, err := roleService.Create(ctx, "default", "Admin", nil)
	require.NoError(t, err)
	return assignmentService, roleService, viewer, admin, ctx
}

func TestAssignmentService_AssignAndRolesForSubject(t *testing.T) {
	service, _, viewer, admin, ctx := newAssignmentFixture(t)

	require.NoError(t, service.Assign(ctx, "default", "user@example.com", viewer.ID))
	require.NoError(t, service.Assign(ctx, "default", "user@example.com", admin.ID))

	roleNames, err := service.RolesForSubject(ctx, "default", "user@example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"Admin", "Viewer"}, roleNames)

	roleNames, err = service.RolesForSubject(ctx, "default", "nobody")
	require.NoError(t, err)
	assert.Empty(t, roleNames)
}

func TestAssignmentService_UnknownRole(t *testing.T) {
	service, _, _, _, ctx := newAssignmentFixture(t)

	err := service.Assign(ctx, "default", "user@example.com", "0b6ef1f6-0000-0000-0000-000000000000")
	var notFound RoleNotFoundError
	assert.ErrorAs(t, err, &notFound)

	err = service.Assign(ctx, "default", "user@example.com", "not-a-uuid")
	var invalid InvalidAssignmentError
	assert.ErrorAs(t, err, &invalid)
}

func TestAssignmentService_Duplicate(t *testing.T) {
	service, _, viewer, _, ctx := newAssignmentFixture(t)

	require.NoError(t, service.Assign(ctx, "default", "user@example.com", viewer.ID))
	err := service.Assign(ctx, "default", "user@example.com", viewer.ID)
	var duplicate AssignmentDuplicateError
	assert.ErrorAs(t, err, &duplicate)
}

func TestAssignmentService_ListFilters(t *testing.T) {
	service, _, viewer, admin, ctx := newAssignmentFixture(t)

	require.NoError(t, service.Assign(ctx, "default", "user-1", viewer.ID))
	require.NoError(t, service.Assign(ctx, "default", "user-1", admin.ID))
	require.NoError(t, service.Assign(ctx, "default", "user-2", viewer.ID))

	all, err := service.List(ctx, "default", "", "")
	require.NoError(t, err)
	assert.Len(t, all, 3)

	bySubject, err := service.List(ctx, "default", "user-1", "")
	require.NoError(t, err)
	assert.Len(t, bySubject, 2)

	byRole, err := service.List(ctx, "default", "", viewer.ID)
	require.NoError(t, err)
	assert.Len(t, byRole, 2)

	both, err := service.List(ctx, "default", "user-2", viewer.ID)
	require.NoError(t, err)
	assert.Len(t, both, 1)
}

func TestAssignmentService_Unassign(t *testing.T) {
	service, _, viewer, _, ctx := newAssignmentFixture(t)

	require.NoError(t, service.Assign(ctx, "default", "user-1", viewer.ID))
	require.NoError(t, service.Unassign(ctx, "default", "user-1", viewer.ID))

	err := service.Unassign(ctx, "default", "user-1", viewer.ID)
	var notFound AssignmentNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestAssignmentService_CrossTenantRoleRejected(t *testing.T) {
	service, _, viewer, _, ctx := newAssignmentFixture(t)

	// viewer belongs to "default"; assigning it in another tenant must fail
	// even though the role id is real (composite FK on tenant_id + role_id)
	err := service.Assign(ctx, "other", "mallory", viewer.ID)
	var notFound RoleNotFoundError
	assert.ErrorAs(t, err, &notFound)

	roleNames, err := service.RolesForSubject(ctx, "other", "mallory")
	require.NoError(t, err)
	assert.Empty(t, roleNames)
}

func TestAssignmentService_CascadeOnRoleDelete(t *testing.T) {
	service, roleService, viewer, _, ctx := newAssignmentFixture(t)

	require.NoError(t, service.Assign(ctx, "default", "user-1", viewer.ID))
	require.NoError(t, roleService.Delete(ctx, "default", viewer.ID))

	roleNames, err := service.RolesForSubject(ctx, "default", "user-1")
	require.NoError(t, err)
	assert.Empty(t, roleNames)
}
