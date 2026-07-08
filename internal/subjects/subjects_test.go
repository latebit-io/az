package subjects

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/latebit-io/az/internal/resources"
	"github.com/latebit-io/az/internal/roles"
	"github.com/latebit-io/az/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	os.Exit(utils.RunTestMain(m))
}

func newSubjectService(pool *pgxpool.Pool) SubjectService {
	return NewDefaultSubjectService(NewPostgresSubjectRepository(pool),
		roles.NewPostgresAssignmentRepository(pool), utils.NewPostgresTxManager(pool))
}

func TestSubjectService_CreateAndGet(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := newSubjectService(pool)
	ctx := context.Background()

	err := service.Create(ctx, "default", "user@example.com", "user@example.com",
		map[string]any{"department": "engineering"})
	require.NoError(t, err)

	subject, err := service.Get(ctx, "default", "user@example.com")
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", subject.Key)
	assert.Equal(t, "user@example.com", subject.Email)
	assert.Equal(t, map[string]any{"department": "engineering"}, subject.Attributes)
}

func TestSubjectService_Validation(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := newSubjectService(pool)
	ctx := context.Background()

	tests := []struct {
		name  string
		key   string
		email string
	}{
		{"empty key", "", ""},
		{"key too long", strings.Repeat("a", 255), ""},
		{"bad email", "user-1", "not-an-email"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.Create(ctx, "default", tt.key, tt.email, nil)
			var invalid InvalidSubjectError
			assert.ErrorAs(t, err, &invalid)
		})
	}
}

func TestSubjectService_Duplicate(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := newSubjectService(pool)
	ctx := context.Background()

	require.NoError(t, service.Create(ctx, "default", "user-1", "", nil))
	err := service.Create(ctx, "default", "user-1", "", nil)
	var duplicate SubjectDuplicateError
	assert.ErrorAs(t, err, &duplicate)

	// same key in another tenant is fine
	assert.NoError(t, service.Create(ctx, "other", "user-1", "", nil))
}

func TestSubjectService_ListUpdateDelete(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := newSubjectService(pool)
	ctx := context.Background()

	require.NoError(t, service.Create(ctx, "default", "user-1", "", map[string]any{"level": float64(1)}))
	require.NoError(t, service.Create(ctx, "default", "user-2", "", nil))

	subjectList, err := service.List(ctx, "default")
	require.NoError(t, err)
	assert.Len(t, subjectList, 2)

	require.NoError(t, service.Update(ctx, "default", "user-1", "user@example.com", map[string]any{"level": float64(2)}))
	subject, err := service.Get(ctx, "default", "user-1")
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", subject.Email)
	assert.Equal(t, map[string]any{"level": float64(2)}, subject.Attributes)

	require.NoError(t, service.Delete(ctx, "default", "user-1"))
	_, err = service.Get(ctx, "default", "user-1")
	var notFound SubjectNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestSubjectService_DeleteCascadesAssignments(t *testing.T) {
	pool := utils.NewTestPool(t)
	service := newSubjectService(pool)
	resourceService := resources.NewDefaultResourceService(resources.NewPostgresResourceTypeRepository(pool))
	roleService := roles.NewDefaultRoleService(roles.NewPostgresRoleRepository(pool), resourceService)
	assignmentService := roles.NewDefaultAssignmentService(roles.NewPostgresAssignmentRepository(pool))
	ctx := context.Background()

	require.NoError(t, service.Create(ctx, "default", "user-1", "", nil))
	require.NoError(t, roleService.Create(ctx, "default", "viewer", "Viewer", "", nil))
	require.NoError(t, assignmentService.Assign(ctx, "default", "user-1", "viewer"))

	require.NoError(t, service.Delete(ctx, "default", "user-1"))

	roleKeys, err := assignmentService.RolesForSubject(ctx, "default", "user-1")
	require.NoError(t, err)
	assert.Empty(t, roleKeys)
}
