package check

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/latebit-io/az/internal/resources"
	"github.com/latebit-io/az/internal/roles"
	"github.com/latebit-io/az/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	os.Exit(utils.RunTestMain(m))
}

// newCheckFixture seeds a policy world:
//
//	resource type document (read/write)
//	resource type report (read)
//	role viewer -> document:read
//	role editor -> document:read, document:write
//	alice: editor
//	bob:   viewer
func newCheckFixture(t *testing.T) (CheckService, context.Context) {
	pool := utils.NewTestPool(t)
	ctx := context.Background()

	resourceRepo := resources.NewPostgresResourceTypeRepository(pool)
	resourceService := resources.NewDefaultResourceService(resourceRepo, utils.NewPostgresTxManager(pool))
	roleRepo := roles.NewPostgresRoleRepository(pool)
	roleService := roles.NewDefaultRoleService(roleRepo, utils.NewPostgresTxManager(pool))
	assignmentRepo := roles.NewPostgresAssignmentRepository(pool)
	assignmentService := roles.NewDefaultAssignmentService(assignmentRepo)

	require.NoError(t, resourceService.Create(ctx, "default", "document", []string{"read", "write"}))
	require.NoError(t, resourceService.Create(ctx, "default", "report", []string{"read"}))

	require.NoError(t, roleService.Create(ctx, "default", "viewer", "Viewer", "",
		[]roles.Permission{{Resource: "document", Action: "read"}}))
	require.NoError(t, roleService.Create(ctx, "default", "editor", "Editor", "",
		[]roles.Permission{{Resource: "document", Action: "read"}, {Resource: "document", Action: "write"}}))

	require.NoError(t, assignmentService.Assign(ctx, "default", "alice", "editor"))
	require.NoError(t, assignmentService.Assign(ctx, "default", "bob", "viewer"))

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	checkService := NewDefaultCheckService(resourceRepo, roleRepo, assignmentRepo, logger, true)
	return checkService, ctx
}

func TestCheck_Decisions(t *testing.T) {
	service, ctx := newCheckFixture(t)

	tests := []struct {
		name    string
		request CheckRequest
		allow   bool
	}{
		{"allow editor write", CheckRequest{Subject: "alice", Action: "write", Resource: "document"}, true},
		{"allow editor read", CheckRequest{Subject: "alice", Action: "read", Resource: "document"}, true},
		{"allow viewer read", CheckRequest{Subject: "bob", Action: "read", Resource: "document"}, true},
		{"deny viewer write", CheckRequest{Subject: "bob", Action: "write", Resource: "document"}, false},
		{"deny unknown subject", CheckRequest{Subject: "nobody", Action: "read", Resource: "document"}, false},
		{"deny unknown resource type", CheckRequest{Subject: "alice", Action: "read", Resource: "spaceship"}, false},
		{"deny unknown action", CheckRequest{Subject: "alice", Action: "share", Resource: "document"}, false},
		{"deny no grant on other type", CheckRequest{Subject: "alice", Action: "read", Resource: "report"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := service.Check(ctx, "default", tt.request)
			require.NoError(t, err)
			assert.Equal(t, tt.allow, decision.Allow, "reason: %s", decision.Reason)
			assert.NotEmpty(t, decision.Reason)
		})
	}
}

func TestCheck_TenantIsolation(t *testing.T) {
	service, ctx := newCheckFixture(t)

	decision, err := service.Check(ctx, "other", CheckRequest{Subject: "alice", Action: "write",
		Resource: "document"})
	require.NoError(t, err)
	assert.False(t, decision.Allow)
	assert.Contains(t, decision.Reason, "unknown resource type")
}

func TestCheck_InvalidRequests(t *testing.T) {
	service, ctx := newCheckFixture(t)

	tests := []struct {
		name    string
		request CheckRequest
	}{
		{"missing subject", CheckRequest{Action: "read", Resource: "document"}},
		{"missing action", CheckRequest{Subject: "alice", Resource: "document"}},
		{"missing resource", CheckRequest{Subject: "alice", Action: "read"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Check(ctx, "default", tt.request)
			var invalid InvalidCheckError
			assert.ErrorAs(t, err, &invalid)
		})
	}
}

func TestCheck_Bulk(t *testing.T) {
	service, ctx := newCheckFixture(t)

	decisions, err := service.CheckBulk(ctx, "default", []CheckRequest{
		{Subject: "alice", Action: "write", Resource: "document"},
		{Subject: "bob", Action: "write", Resource: "document"},
		{Subject: "bob", Action: "read", Resource: "document"},
	})
	require.NoError(t, err)
	require.Len(t, decisions, 3)
	assert.True(t, decisions[0].Allow)
	assert.False(t, decisions[1].Allow)
	assert.True(t, decisions[2].Allow)
}

func TestCheck_ReasonMentionsGrantingRole(t *testing.T) {
	service, ctx := newCheckFixture(t)

	decision, err := service.Check(ctx, "default", CheckRequest{Subject: "alice", Action: "write",
		Resource: "document"})
	require.NoError(t, err)
	assert.True(t, decision.Allow)
	assert.Contains(t, decision.Reason, "editor")
}
