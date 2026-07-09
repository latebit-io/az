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

func TestResourceService_CreateAndGet(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool))
	ctx := context.Background()

	err := service.Create(ctx, "default", "document", []string{"read", "write", "delete"})
	require.NoError(t, err)

	resourceType, err := service.Get(ctx, "default", "document")
	require.NoError(t, err)
	assert.Equal(t, "document", resourceType.Key)
	assert.Equal(t, []string{"read", "write", "delete"}, resourceType.Actions)
	assert.True(t, resourceType.HasAction("read"))
	assert.False(t, resourceType.HasAction("share"))
	assert.NotEmpty(t, resourceType.ID)
}

func TestResourceService_CreateValidation(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool))
	ctx := context.Background()

	tests := []struct {
		name    string
		key     string
		actions []string
	}{
		{"invalid key", "Bad Key", []string{"read"}},
		{"no actions", "document", nil},
		{"invalid action", "document", []string{"Read It"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.Create(ctx, "default", tt.key, tt.actions)
			var invalid InvalidResourceTypeError
			assert.ErrorAs(t, err, &invalid)
		})
	}
}

func TestResourceService_Duplicate(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool))
	ctx := context.Background()

	require.NoError(t, service.Create(ctx, "default", "document", []string{"read"}))
	err := service.Create(ctx, "default", "document", []string{"read"})
	var duplicate ResourceTypeDuplicateError
	assert.ErrorAs(t, err, &duplicate)

	// same key in another tenant is fine
	assert.NoError(t, service.Create(ctx, "other", "document", []string{"read"}))
}

func TestResourceService_List(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool))
	ctx := context.Background()

	require.NoError(t, service.Create(ctx, "default", "document", []string{"read"}))
	require.NoError(t, service.Create(ctx, "default", "account", []string{"read"}))
	require.NoError(t, service.Create(ctx, "other", "widget", []string{"read"}))

	resourceTypes, err := service.List(ctx, "default")
	require.NoError(t, err)
	require.Len(t, resourceTypes, 2)
	assert.Equal(t, "account", resourceTypes[0].Key)
	assert.Equal(t, "document", resourceTypes[1].Key)
}

func TestResourceService_Update(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool))
	ctx := context.Background()

	require.NoError(t, service.Create(ctx, "default", "document", []string{"read"}))
	err := service.Update(ctx, "default", "document", []string{"read", "write"})
	require.NoError(t, err)

	resourceType, err := service.Get(ctx, "default", "document")
	require.NoError(t, err)
	assert.Equal(t, []string{"read", "write"}, resourceType.Actions)

	err = service.Update(ctx, "default", "missing", []string{"read"})
	var notFound ResourceTypeNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestResourceService_Delete(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool))
	ctx := context.Background()

	require.NoError(t, service.Create(ctx, "default", "document", []string{"read"}))
	require.NoError(t, service.Delete(ctx, "default", "document"))

	_, err := service.Get(ctx, "default", "document")
	var notFound ResourceTypeNotFoundError
	assert.ErrorAs(t, err, &notFound)

	err = service.Delete(ctx, "default", "document")
	assert.ErrorAs(t, err, &notFound)
}
