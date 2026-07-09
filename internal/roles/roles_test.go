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

func newRoleFixture(t *testing.T) (RoleService, resources.ResourceService, context.Context) {
	pool := utils.NewTestPool(t)
	resourceService := resources.NewDefaultResourceService(resources.NewPostgresResourceTypeRepository(pool), utils.NewPostgresTxManager(pool))
	roleService := NewDefaultRoleService(NewPostgresRoleRepository(pool), utils.NewPostgresTxManager(pool))
	ctx := context.Background()
	require.NoError(t, resourceService.Create(ctx, "default", "document", []string{"read", "write", "delete"}))
	return roleService, resourceService, ctx
}

func TestRoleService_CreateAndGet(t *testing.T) {
	service, _, ctx := newRoleFixture(t)

	err := service.Create(ctx, "default", "editor", "Editor", "can edit documents",
		[]Permission{{Resource: "document", Action: "read"}, {Resource: "document", Action: "write"}})
	require.NoError(t, err)

	role, err := service.Get(ctx, "default", "editor")
	require.NoError(t, err)
	assert.Equal(t, "editor", role.Key)
	assert.Equal(t, []Permission{{Resource: "document", Action: "read"}, {Resource: "document", Action: "write"}},
		role.Permissions)
}

func TestRoleService_GrantValidation(t *testing.T) {
	service, _, ctx := newRoleFixture(t)

	err := service.Create(ctx, "default", "editor", "Editor", "",
		[]Permission{{Resource: "missing", Action: "read"}})
	var invalidPermission InvalidPermissionError
	require.ErrorAs(t, err, &invalidPermission)
	assert.Equal(t, "missing", invalidPermission.Resource)

	err = service.Create(ctx, "default", "editor", "Editor", "",
		[]Permission{{Resource: "document", Action: "share"}})
	require.ErrorAs(t, err, &invalidPermission)
	assert.Equal(t, "share", invalidPermission.Action)

	// a failed create must not leave a partial role behind
	_, err = service.Get(ctx, "default", "editor")
	var notFound RoleNotFoundError
	assert.ErrorAs(t, err, &notFound)

	err = service.Create(ctx, "default", "Bad Key", "Editor", "", nil)
	var invalidRole InvalidRoleError
	assert.ErrorAs(t, err, &invalidRole)

	err = service.Create(ctx, "default", "editor", "", "", nil)
	assert.ErrorAs(t, err, &invalidRole)
}

func TestRoleService_Duplicate(t *testing.T) {
	service, _, ctx := newRoleFixture(t)

	require.NoError(t, service.Create(ctx, "default", "editor", "Editor", "", nil))
	err := service.Create(ctx, "default", "editor", "Editor", "", nil)
	var duplicate RoleDuplicateError
	assert.ErrorAs(t, err, &duplicate)
}

func TestRoleService_ListUpdateDelete(t *testing.T) {
	service, _, ctx := newRoleFixture(t)

	require.NoError(t, service.Create(ctx, "default", "editor", "Editor", "", nil))
	require.NoError(t, service.Create(ctx, "default", "admin", "Admin", "", nil))

	roleList, err := service.List(ctx, "default")
	require.NoError(t, err)
	require.Len(t, roleList, 2)
	assert.Equal(t, "admin", roleList[0].Key)

	err = service.Update(ctx, "default", "editor", "Editors", "updated",
		[]Permission{{Resource: "document", Action: "read"}})
	require.NoError(t, err)
	role, err := service.Get(ctx, "default", "editor")
	require.NoError(t, err)
	assert.Equal(t, "Editors", role.Name)
	assert.Len(t, role.Permissions, 1)

	require.NoError(t, service.Delete(ctx, "default", "editor"))
	_, err = service.Get(ctx, "default", "editor")
	var notFound RoleNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestResourceTypeDeleteBlockedByRoleReference(t *testing.T) {
	pool := utils.NewTestPool(t)
	roleRepo := NewPostgresRoleRepository(pool)
	resourceService := resources.NewDefaultResourceService(resources.NewPostgresResourceTypeRepository(pool),
		utils.NewPostgresTxManager(pool))
	roleService := NewDefaultRoleService(roleRepo, utils.NewPostgresTxManager(pool))
	ctx := context.Background()

	require.NoError(t, resourceService.Create(ctx, "default", "document", []string{"read"}))
	require.NoError(t, roleService.Create(ctx, "default", "viewer", "Viewer", "",
		[]Permission{{Resource: "document", Action: "read"}}))

	err := resourceService.Delete(ctx, "default", "document")
	var referenced resources.ResourceTypeReferencedError
	assert.ErrorAs(t, err, &referenced)

	require.NoError(t, roleService.Delete(ctx, "default", "viewer"))
	assert.NoError(t, resourceService.Delete(ctx, "default", "document"))
}

func TestResourceTypeActionRemovalBlockedByRoleReference(t *testing.T) {
	pool := utils.NewTestPool(t)
	resourceService := resources.NewDefaultResourceService(resources.NewPostgresResourceTypeRepository(pool),
		utils.NewPostgresTxManager(pool))
	roleService := NewDefaultRoleService(NewPostgresRoleRepository(pool), utils.NewPostgresTxManager(pool))
	ctx := context.Background()

	require.NoError(t, resourceService.Create(ctx, "default", "document", []string{"read", "write"}))
	require.NoError(t, roleService.Create(ctx, "default", "editor", "Editor", "",
		[]Permission{{Resource: "document", Action: "write"}}))

	// removing the granted action is refused
	err := resourceService.Update(ctx, "default", "document", []string{"read"})
	var referenced resources.ResourceTypeReferencedError
	assert.ErrorAs(t, err, &referenced)

	// removing an ungranted action is fine
	assert.NoError(t, resourceService.Update(ctx, "default", "document", []string{"write"}))
}

func TestRoleRepository_AnyGrants(t *testing.T) {
	pool := utils.NewTestPool(t)
	resourceService := resources.NewDefaultResourceService(resources.NewPostgresResourceTypeRepository(pool), utils.NewPostgresTxManager(pool))
	repo := NewPostgresRoleRepository(pool)
	service := NewDefaultRoleService(repo, utils.NewPostgresTxManager(pool))
	ctx := context.Background()

	require.NoError(t, resourceService.Create(ctx, "default", "document", []string{"read", "write"}))
	require.NoError(t, service.Create(ctx, "default", "viewer", "Viewer", "",
		[]Permission{{Resource: "document", Action: "read"}}))
	require.NoError(t, service.Create(ctx, "default", "editor", "Editor", "",
		[]Permission{{Resource: "document", Action: "read"}, {Resource: "document", Action: "write"}}))

	roleKey, granted, err := repo.AnyGrants(ctx, "default", []string{"viewer", "editor"}, "document", "write")
	require.NoError(t, err)
	assert.True(t, granted)
	assert.Equal(t, "editor", roleKey)

	_, granted, err = repo.AnyGrants(ctx, "default", []string{"viewer"}, "document", "write")
	require.NoError(t, err)
	assert.False(t, granted)

	_, granted, err = repo.AnyGrants(ctx, "default", nil, "document", "read")
	require.NoError(t, err)
	assert.False(t, granted)

	// tenant isolation
	_, granted, err = repo.AnyGrants(ctx, "other", []string{"editor"}, "document", "write")
	require.NoError(t, err)
	assert.False(t, granted)
}
