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
	assignmentService := roles.NewDefaultAssignmentService(roles.NewPostgresAssignmentRepository(pool))

	{ _, err := resourceService.Create(ctx, "default", "document", []string{"read", "write"}); require.NoError(t, err) }
	{ _, err := resourceService.Create(ctx, "default", "report", []string{"read"}); require.NoError(t, err) }

	viewer, err := roleService.Create(ctx, "default", "viewer",
		[]roles.Permission{{Resource: "document", Action: "read"}})
	require.NoError(t, err)
	editor, err := roleService.Create(ctx, "default", "editor",
		[]roles.Permission{{Resource: "document", Action: "read"}, {Resource: "document", Action: "write"}})
	require.NoError(t, err)

	require.NoError(t, assignmentService.Assign(ctx, "default", "alice", editor.ID))
	require.NoError(t, assignmentService.Assign(ctx, "default", "bob", viewer.ID))

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	checkService := NewDefaultCheckService(NewPostgresCheckRepository(pool), logger, true)
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

// TestCheckRepository_Resolve exercises the single-round-trip query directly:
// alice holds two roles and only one of them grants write, so the lateral join
// has to pick the granting role rather than the first assigned one.
func TestCheckRepository_Resolve(t *testing.T) {
	pool := utils.NewTestPool(t)
	ctx := context.Background()
	txManager := utils.NewPostgresTxManager(pool)
	resourceService := resources.NewDefaultResourceService(resources.NewPostgresResourceTypeRepository(pool),
		txManager)
	roleService := roles.NewDefaultRoleService(roles.NewPostgresRoleRepository(pool), txManager)
	assignmentService := roles.NewDefaultAssignmentService(roles.NewPostgresAssignmentRepository(pool))

	_, err := resourceService.Create(ctx, "default", "document", []string{"read", "write"})
	require.NoError(t, err)
	viewer, err := roleService.Create(ctx, "default", "Viewer",
		[]roles.Permission{{Resource: "document", Action: "read"}})
	require.NoError(t, err)
	editor, err := roleService.Create(ctx, "default", "Editor",
		[]roles.Permission{{Resource: "document", Action: "read"}, {Resource: "document", Action: "write"}})
	require.NoError(t, err)

	require.NoError(t, assignmentService.Assign(ctx, "default", "alice", viewer.ID))
	require.NoError(t, assignmentService.Assign(ctx, "default", "alice", editor.ID))
	require.NoError(t, assignmentService.Assign(ctx, "default", "bob", viewer.ID))

	repo := NewPostgresCheckRepository(pool)

	tests := []struct {
		name     string
		tenantID string
		request  CheckRequest
		expected Grant
	}{
		{"granting role wins over other assignments", "default",
			CheckRequest{Subject: "alice", Action: "write", Resource: "document"},
			Grant{TypeFound: true, ActionDeclared: true, Granted: true, RoleName: "Editor"}},
		{"assigned but ungranted action", "default",
			CheckRequest{Subject: "bob", Action: "write", Resource: "document"},
			Grant{TypeFound: true, ActionDeclared: true}},
		{"unassigned subject", "default",
			CheckRequest{Subject: "nobody", Action: "read", Resource: "document"},
			Grant{TypeFound: true, ActionDeclared: true}},
		{"undeclared action", "default",
			CheckRequest{Subject: "alice", Action: "share", Resource: "document"},
			Grant{TypeFound: true}},
		{"unknown resource type", "default",
			CheckRequest{Subject: "alice", Action: "read", Resource: "spaceship"},
			Grant{}},
		{"tenant isolation", "other",
			CheckRequest{Subject: "alice", Action: "write", Resource: "document"},
			Grant{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grant, err := repo.Resolve(ctx, tt.tenantID, tt.request)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, grant)
		})
	}
}
