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

	err := service.Create(ctx, "default", "document", "Document", "documents", []string{"read", "write", "delete"},
		[]AttributeDef{{Key: "public", Type: AttributeTypeBool}})
	require.NoError(t, err)

	resourceType, err := service.Get(ctx, "default", "document")
	require.NoError(t, err)
	assert.Equal(t, "document", resourceType.Key)
	assert.Equal(t, "Document", resourceType.Name)
	assert.Equal(t, []string{"read", "write", "delete"}, resourceType.Actions)
	assert.Equal(t, []AttributeDef{{Key: "public", Type: AttributeTypeBool}}, resourceType.Attributes)
	assert.True(t, resourceType.HasAction("read"))
	assert.False(t, resourceType.HasAction("share"))
	assert.NotEmpty(t, resourceType.ID)
}

func TestResourceService_CreateValidation(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool))
	ctx := context.Background()

	tests := []struct {
		name       string
		key        string
		typeName   string
		actions    []string
		attributes []AttributeDef
	}{
		{"invalid key", "Bad Key", "Name", []string{"read"}, nil},
		{"empty name", "document", "", []string{"read"}, nil},
		{"no actions", "document", "Name", nil, nil},
		{"invalid action", "document", "Name", []string{"Read It"}, nil},
		{"invalid attribute key", "document", "Name", []string{"read"}, []AttributeDef{{Key: "Bad Key", Type: "string"}}},
		{"invalid attribute type", "document", "Name", []string{"read"}, []AttributeDef{{Key: "public", Type: "object"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.Create(ctx, "default", tt.key, tt.typeName, "", tt.actions, tt.attributes)
			var invalid InvalidResourceTypeError
			assert.ErrorAs(t, err, &invalid)
		})
	}
}

func TestResourceService_Duplicate(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool))
	ctx := context.Background()

	require.NoError(t, service.Create(ctx, "default", "document", "Document", "", []string{"read"}, nil))
	err := service.Create(ctx, "default", "document", "Document", "", []string{"read"}, nil)
	var duplicate ResourceTypeDuplicateError
	assert.ErrorAs(t, err, &duplicate)

	// same key in another tenant is fine
	assert.NoError(t, service.Create(ctx, "other", "document", "Document", "", []string{"read"}, nil))
}

func TestResourceService_List(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool))
	ctx := context.Background()

	require.NoError(t, service.Create(ctx, "default", "document", "Document", "", []string{"read"}, nil))
	require.NoError(t, service.Create(ctx, "default", "account", "Account", "", []string{"read"}, nil))
	require.NoError(t, service.Create(ctx, "other", "widget", "Widget", "", []string{"read"}, nil))

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

	require.NoError(t, service.Create(ctx, "default", "document", "Document", "", []string{"read"}, nil))
	err := service.Update(ctx, "default", "document", "Documents", "all documents", []string{"read", "write"}, nil)
	require.NoError(t, err)

	resourceType, err := service.Get(ctx, "default", "document")
	require.NoError(t, err)
	assert.Equal(t, "Documents", resourceType.Name)
	assert.Equal(t, []string{"read", "write"}, resourceType.Actions)

	err = service.Update(ctx, "default", "missing", "Name", "", []string{"read"}, nil)
	var notFound ResourceTypeNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestResourceService_Delete(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := NewDefaultResourceService(NewPostgresResourceTypeRepository(pool))
	ctx := context.Background()

	require.NoError(t, service.Create(ctx, "default", "document", "Document", "", []string{"read"}, nil))
	require.NoError(t, service.Delete(ctx, "default", "document"))

	_, err := service.Get(ctx, "default", "document")
	var notFound ResourceTypeNotFoundError
	assert.ErrorAs(t, err, &notFound)

	err = service.Delete(ctx, "default", "document")
	assert.ErrorAs(t, err, &notFound)
}
