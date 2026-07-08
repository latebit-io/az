package resources

import (
	"context"
	"testing"

	"github.com/latebit-io/az/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceService_CreateAndGet(t *testing.T) {
	pool := utils.NewTestPool(t)
	resourceService := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool))
	service := NewDefaultInstanceService(NewPostgresInstanceRepository(pool))
	ctx := context.Background()

	require.NoError(t, resourceService.Create(ctx, "default", "document", "Document", "", []string{"read"}, nil))

	err := service.Create(ctx, "default", "document", "doc-1", map[string]any{"public": true, "pages": float64(3)})
	require.NoError(t, err)

	instance, err := service.Get(ctx, "default", "document", "doc-1")
	require.NoError(t, err)
	assert.Equal(t, "doc-1", instance.Key)
	assert.Equal(t, map[string]any{"public": true, "pages": float64(3)}, instance.Attributes)
}

func TestInstanceService_UnknownResourceType(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := NewDefaultInstanceService(NewPostgresInstanceRepository(pool))
	ctx := context.Background()

	err := service.Create(ctx, "default", "missing", "doc-1", nil)
	var notFound ResourceTypeNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestInstanceService_Duplicate(t *testing.T) {
	pool := utils.NewTestPool(t)
	resourceService := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool))
	service := NewDefaultInstanceService(NewPostgresInstanceRepository(pool))
	ctx := context.Background()

	require.NoError(t, resourceService.Create(ctx, "default", "document", "Document", "", []string{"read"}, nil))
	require.NoError(t, service.Create(ctx, "default", "document", "doc-1", nil))

	err := service.Create(ctx, "default", "document", "doc-1", nil)
	var duplicate InstanceDuplicateError
	assert.ErrorAs(t, err, &duplicate)
}

func TestInstanceService_UpdateAndDelete(t *testing.T) {
	pool := utils.NewTestPool(t)
	resourceService := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool))
	service := NewDefaultInstanceService(NewPostgresInstanceRepository(pool))
	ctx := context.Background()

	require.NoError(t, resourceService.Create(ctx, "default", "document", "Document", "", []string{"read"}, nil))
	require.NoError(t, service.Create(ctx, "default", "document", "doc-1", map[string]any{"public": false}))

	require.NoError(t, service.Update(ctx, "default", "document", "doc-1", map[string]any{"public": true}))
	instance, err := service.Get(ctx, "default", "document", "doc-1")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"public": true}, instance.Attributes)

	require.NoError(t, service.Delete(ctx, "default", "document", "doc-1"))
	_, err = service.Get(ctx, "default", "document", "doc-1")
	var notFound InstanceNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestInstanceService_CascadeOnResourceTypeDelete(t *testing.T) {
	pool := utils.NewTestPool(t)
	resourceService := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool))
	service := NewDefaultInstanceService(NewPostgresInstanceRepository(pool))
	ctx := context.Background()

	require.NoError(t, resourceService.Create(ctx, "default", "document", "Document", "", []string{"read"}, nil))
	require.NoError(t, service.Create(ctx, "default", "document", "doc-1", nil))

	require.NoError(t, resourceService.Delete(ctx, "default", "document"))
	_, err := service.Get(ctx, "default", "document", "doc-1")
	var notFound InstanceNotFoundError
	assert.ErrorAs(t, err, &notFound)
}
