package resources

import (
	"context"
	"os"
	"testing"

	"github.com/latebit-io/az/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	os.Exit(utils.RunTestMain(m))
}

func newResourceService(t *testing.T) (ResourceService, context.Context) {
	pool := utils.NewTestPool(t)
	service := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool), utils.NewPostgresTxManager(pool))
	return service, context.Background()
}

func TestResourceService_CreateAndGet(t *testing.T) {
	service, ctx := newResourceService(t)

	created, err := service.Create(ctx, "default", "document", []string{"read", "write", "delete"})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)

	resourceType, err := service.Get(ctx, "default", created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, resourceType.ID)
	assert.Equal(t, "document", resourceType.Name)
	assert.ElementsMatch(t, []string{"read", "write", "delete"}, resourceType.Actions)
	assert.True(t, resourceType.HasAction("read"))
	assert.False(t, resourceType.HasAction("share"))

	_, err = service.Get(ctx, "default", "not-a-uuid")
	var invalid InvalidResourceTypeError
	assert.ErrorAs(t, err, &invalid)
}

func TestResourceService_CreateValidation(t *testing.T) {
	service, ctx := newResourceService(t)

	tests := []struct {
		name     string
		typeName string
		actions  []string
	}{
		{"invalid name", "Bad Name", []string{"read"}},
		{"no actions", "document", nil},
		{"invalid action", "document", []string{"Read It"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Create(ctx, "default", tt.typeName, tt.actions)
			var invalid InvalidResourceTypeError
			assert.ErrorAs(t, err, &invalid)
		})
	}
}

func TestResourceService_DuplicateName(t *testing.T) {
	service, ctx := newResourceService(t)

	_, err := service.Create(ctx, "default", "document", []string{"read"})
	require.NoError(t, err)
	_, err = service.Create(ctx, "default", "document", []string{"read"})
	var duplicate ResourceTypeDuplicateError
	assert.ErrorAs(t, err, &duplicate)

	// same name in another tenant is fine
	_, err = service.Create(ctx, "other", "document", []string{"read"})
	assert.NoError(t, err)
}

func TestResourceService_List(t *testing.T) {
	service, ctx := newResourceService(t)

	_, err := service.Create(ctx, "default", "document", []string{"read"})
	require.NoError(t, err)
	_, err = service.Create(ctx, "default", "account", []string{"read"})
	require.NoError(t, err)
	_, err = service.Create(ctx, "other", "widget", []string{"read"})
	require.NoError(t, err)

	resourceTypes, err := service.List(ctx, "default")
	require.NoError(t, err)
	require.Len(t, resourceTypes, 2)
	assert.Equal(t, "account", resourceTypes[0].Name)
	assert.Equal(t, "document", resourceTypes[1].Name)
}

func TestResourceService_Update(t *testing.T) {
	service, ctx := newResourceService(t)

	created, err := service.Create(ctx, "default", "document", []string{"read"})
	require.NoError(t, err)

	err = service.Update(ctx, "default", created.ID, "documents", []string{"read", "write"})
	require.NoError(t, err)

	resourceType, err := service.Get(ctx, "default", created.ID)
	require.NoError(t, err)
	assert.Equal(t, "documents", resourceType.Name)
	assert.Equal(t, []string{"read", "write"}, resourceType.Actions)

	err = service.Update(ctx, "default", "0b6ef1f6-0000-0000-0000-000000000000", "name", []string{"read"})
	var notFound ResourceTypeNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestResourceService_Delete(t *testing.T) {
	service, ctx := newResourceService(t)

	created, err := service.Create(ctx, "default", "document", []string{"read"})
	require.NoError(t, err)
	require.NoError(t, service.Delete(ctx, "default", created.ID))

	_, err = service.Get(ctx, "default", created.ID)
	var notFound ResourceTypeNotFoundError
	assert.ErrorAs(t, err, &notFound)

	err = service.Delete(ctx, "default", created.ID)
	assert.ErrorAs(t, err, &notFound)
}
