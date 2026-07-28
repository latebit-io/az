package roles

import (
	"context"
	"os"
	"testing"

	"github.com/latebit-io/az/internal/resources"
	"github.com/latebit-io/az/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	os.Exit(utils.RunTestMain(m))
}

func newRoleFixture(t *testing.T) (RoleService, resources.ResourceService, *resources.ResourceType, context.Context) {
	pool := utils.NewTestPool(t)
	txManager := utils.NewPostgresTxManager(pool)
	resourceService := resources.NewDefaultResourceService(resources.NewPostgresResourceTypeRepository(pool),
		txManager)
	roleService := NewDefaultRoleService(NewPostgresRoleRepository(pool), txManager)
	ctx := context.Background()
	document, err := resourceService.Create(ctx, "default", "document", []string{"read", "write", "delete"})
	require.NoError(t, err)
	return roleService, resourceService, document, ctx
}

func TestRoleService_CreateAndGet(t *testing.T) {
	service, _, _, ctx := newRoleFixture(t)

	created, err := service.Create(ctx, "default", "Editor",
		[]Permission{{Resource: "document", Action: "read"}, {Resource: "document", Action: "write"}})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)

	role, err := service.Get(ctx, "default", created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, role.ID)
	assert.Equal(t, "Editor", role.Name)
	assert.Equal(t, []Permission{{Resource: "document", Action: "read"}, {Resource: "document", Action: "write"}},
		role.Permissions)
}

func TestRoleService_GrantValidation(t *testing.T) {
	service, _, _, ctx := newRoleFixture(t)

	_, err := service.Create(ctx, "default", "Editor", []Permission{{Resource: "missing", Action: "read"}})
	var invalidPermission InvalidPermissionError
	require.ErrorAs(t, err, &invalidPermission)
	assert.Equal(t, "missing", invalidPermission.Resource)

	_, err = service.Create(ctx, "default", "Editor", []Permission{{Resource: "document", Action: "share"}})
	require.ErrorAs(t, err, &invalidPermission)
	assert.Equal(t, "share", invalidPermission.Action)

	_, err = service.Create(ctx, "default", "", nil)
	var invalidRole InvalidRoleError
	assert.ErrorAs(t, err, &invalidRole)

	// a failed create must not leave a partial role behind
	roleList, err := service.List(ctx, "default")
	require.NoError(t, err)
	assert.Empty(t, roleList)
}

func TestRoleService_DuplicateName(t *testing.T) {
	service, _, _, ctx := newRoleFixture(t)

	_, err := service.Create(ctx, "default", "Editor", nil)
	require.NoError(t, err)
	_, err = service.Create(ctx, "default", "Editor", nil)
	var duplicate RoleDuplicateError
	assert.ErrorAs(t, err, &duplicate)

	// same name in another tenant is fine
	_, err = service.Create(ctx, "other", "Editor", nil)
	assert.NoError(t, err)
}

func TestRoleService_ListUpdateDelete(t *testing.T) {
	service, _, _, ctx := newRoleFixture(t)

	editor, err := service.Create(ctx, "default", "Editor", nil)
	require.NoError(t, err)
	_, err = service.Create(ctx, "default", "Admin", nil)
	require.NoError(t, err)

	roleList, err := service.List(ctx, "default")
	require.NoError(t, err)
	require.Len(t, roleList, 2)
	assert.Equal(t, "Admin", roleList[0].Name)

	err = service.Update(ctx, "default", editor.ID, "Editors",
		[]Permission{{Resource: "document", Action: "read"}})
	require.NoError(t, err)
	role, err := service.Get(ctx, "default", editor.ID)
	require.NoError(t, err)
	assert.Equal(t, "Editors", role.Name)
	assert.Len(t, role.Permissions, 1)

	require.NoError(t, service.Delete(ctx, "default", editor.ID))
	_, err = service.Get(ctx, "default", editor.ID)
	var notFound RoleNotFoundError
	assert.ErrorAs(t, err, &notFound)

	err = service.Delete(ctx, "default", "not-a-uuid")
	var invalidRole InvalidRoleError
	assert.ErrorAs(t, err, &invalidRole)
}

func TestResourceTypeDeleteBlockedByRoleReference(t *testing.T) {
	service, resourceService, document, ctx := newRoleFixture(t)

	viewer, err := service.Create(ctx, "default", "Viewer", []Permission{{Resource: "document", Action: "read"}})
	require.NoError(t, err)

	err = resourceService.Delete(ctx, "default", document.ID)
	var referenced resources.ResourceTypeReferencedError
	assert.ErrorAs(t, err, &referenced)

	require.NoError(t, service.Delete(ctx, "default", viewer.ID))
	assert.NoError(t, resourceService.Delete(ctx, "default", document.ID))
}

func TestResourceTypeActionRemovalBlockedByRoleReference(t *testing.T) {
	service, resourceService, document, ctx := newRoleFixture(t)

	_, err := service.Create(ctx, "default", "Editor", []Permission{{Resource: "document", Action: "write"}})
	require.NoError(t, err)

	// removing the granted action is refused
	err = resourceService.Update(ctx, "default", document.ID, "document", []string{"read"})
	var referenced resources.ResourceTypeReferencedError
	assert.ErrorAs(t, err, &referenced)

	// removing an ungranted action is fine
	assert.NoError(t, resourceService.Update(ctx, "default", document.ID, "document", []string{"write"}))
}
