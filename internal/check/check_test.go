package check

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/latebit-io/az/internal/conditions"
	"github.com/latebit-io/az/internal/resources"
	"github.com/latebit-io/az/internal/roles"
	"github.com/latebit-io/az/internal/subjects"
	"github.com/latebit-io/az/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	os.Exit(utils.RunTestMain(m))
}

// newCheckFixture seeds a full policy world:
//
//	resource type document (read/write) with attribute public
//	resource type report (read)
//	role viewer  -> document:read
//	role editor  -> document:read, document:write
//	alice: editor
//	bob:   viewer
//	subject carol (stored, department=engineering, no roles)
//	instance document/doc-public  (public=true)
//	instance document/doc-private (public=false)
//	subject set engineers  (department == engineering)
//	resource set public-docs (public == true), bound to document
//	rule engineers -> document:read -> public-docs
func newCheckFixture(t *testing.T) (CheckService, context.Context) {
	pool := utils.NewTestPool(t)
	ctx := context.Background()

	resourceRepo := resources.NewPostgresResourceTypeRepository(pool)
	resourceService := resources.NewDefaultResourceService(resourceRepo)
	instanceRepo := resources.NewPostgresInstanceRepository(pool)
	instanceService := resources.NewDefaultInstanceService(instanceRepo)
	subjectRepo := subjects.NewPostgresSubjectRepository(pool)
	subjectService := subjects.NewDefaultSubjectService(subjectRepo,
		roles.NewPostgresAssignmentRepository(pool), utils.NewPostgresTxManager(pool))
	roleRepo := roles.NewPostgresRoleRepository(pool)
	roleService := roles.NewDefaultRoleService(roleRepo, resourceService)
	assignmentRepo := roles.NewPostgresAssignmentRepository(pool)
	assignmentService := roles.NewDefaultAssignmentService(assignmentRepo)
	setRepo := conditions.NewPostgresConditionSetRepository(pool)
	setService := conditions.NewDefaultConditionSetService(setRepo, resourceService)
	ruleRepo := conditions.NewPostgresRuleRepository(pool)
	ruleService := conditions.NewDefaultRuleService(ruleRepo, setRepo, resourceService)

	require.NoError(t, resourceService.Create(ctx, "default", "document", "Document", "",
		[]string{"read", "write"}, []resources.AttributeDef{{Key: "public", Type: resources.AttributeTypeBool}}))
	require.NoError(t, resourceService.Create(ctx, "default", "report", "Report", "", []string{"read"}, nil))

	require.NoError(t, roleService.Create(ctx, "default", "viewer", "Viewer", "",
		[]roles.Permission{{Resource: "document", Action: "read"}}))
	require.NoError(t, roleService.Create(ctx, "default", "editor", "Editor", "",
		[]roles.Permission{{Resource: "document", Action: "read"}, {Resource: "document", Action: "write"}}))

	require.NoError(t, assignmentService.Assign(ctx, "default", "alice", "editor"))
	require.NoError(t, assignmentService.Assign(ctx, "default", "bob", "viewer"))

	require.NoError(t, subjectService.Create(ctx, "default", "carol", "",
		map[string]any{"department": "engineering"}))

	require.NoError(t, instanceService.Create(ctx, "default", "document", "doc-public",
		map[string]any{"public": true}))
	require.NoError(t, instanceService.Create(ctx, "default", "document", "doc-private",
		map[string]any{"public": false}))

	require.NoError(t, setService.Create(ctx, conditions.ConditionSet{
		TenantID: "default", Key: "engineers", Name: "Engineers", Type: conditions.SetTypeSubject,
		Conditions: conditions.ConditionNode{Attribute: "department", Operator: conditions.OperatorEquals,
			Value: "engineering"},
	}))
	require.NoError(t, setService.Create(ctx, conditions.ConditionSet{
		TenantID: "default", Key: "public-docs", Name: "Public documents", Type: conditions.SetTypeResource,
		ResourceType: "document",
		Conditions: conditions.ConditionNode{Attribute: "public", Operator: conditions.OperatorEquals,
			Value: true},
	}))
	require.NoError(t, ruleService.Create(ctx, "default", "engineers", "document", "read", "public-docs"))

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	checkService := NewDefaultCheckService(resourceRepo, instanceRepo, subjectRepo, roleRepo, assignmentRepo,
		setRepo, ruleRepo, logger, true)
	return checkService, ctx
}

func subjectOnly(key string) CheckSubject {
	return CheckSubject{Key: key}
}

func TestCheck_Decisions(t *testing.T) {
	service, ctx := newCheckFixture(t)

	tests := []struct {
		name    string
		request CheckRequest
		allow   bool
	}{
		{"rbac allow editor write", CheckRequest{
			Subject: subjectOnly("alice"), Action: "write",
			Resource: CheckResource{Type: "document"}}, true},
		{"rbac allow viewer read", CheckRequest{
			Subject: subjectOnly("bob"), Action: "read",
			Resource: CheckResource{Type: "document"}}, true},
		{"rbac deny viewer write", CheckRequest{
			Subject: subjectOnly("bob"), Action: "write",
			Resource: CheckResource{Type: "document"}}, false},
		{"deny unknown subject", CheckRequest{
			Subject: subjectOnly("nobody"), Action: "write",
			Resource: CheckResource{Type: "document"}}, false},
		{"deny unknown resource type", CheckRequest{
			Subject: subjectOnly("alice"), Action: "read",
			Resource: CheckResource{Type: "spaceship"}}, false},
		{"deny unknown action", CheckRequest{
			Subject: subjectOnly("alice"), Action: "share",
			Resource: CheckResource{Type: "document"}}, false},
		{"deny role without permission on other type", CheckRequest{
			Subject: subjectOnly("alice"), Action: "read",
			Resource: CheckResource{Type: "report"}}, false},
		{"abac allow stored subject stored public instance", CheckRequest{
			Subject: subjectOnly("carol"), Action: "read",
			Resource: CheckResource{Type: "document", Key: "doc-public"}}, true},
		{"abac deny stored subject private instance", CheckRequest{
			Subject: subjectOnly("carol"), Action: "read",
			Resource: CheckResource{Type: "document", Key: "doc-private"}}, false},
		{"abac deny write not covered by rule", CheckRequest{
			Subject: subjectOnly("carol"), Action: "write",
			Resource: CheckResource{Type: "document", Key: "doc-public"}}, false},
		{"abac allow inline subject attributes", CheckRequest{
			Subject: CheckSubject{Key: "dave", Attributes: map[string]any{"department": "engineering"}},
			Action:  "read",
			Resource: CheckResource{Type: "document",
				Attributes: map[string]any{"public": true}}, // no stored instance either
		}, true},
		{"abac deny inline attrs wrong department", CheckRequest{
			Subject:  CheckSubject{Key: "dave", Attributes: map[string]any{"department": "sales"}},
			Action:   "read",
			Resource: CheckResource{Type: "document", Attributes: map[string]any{"public": true}}}, false},
		{"abac inline overrides stored subject attribute", CheckRequest{
			Subject:  CheckSubject{Key: "carol", Attributes: map[string]any{"department": "sales"}},
			Action:   "read",
			Resource: CheckResource{Type: "document", Key: "doc-public"}}, false},
		{"abac inline overrides stored instance attribute", CheckRequest{
			Subject: subjectOnly("carol"), Action: "read",
			Resource: CheckResource{Type: "document", Key: "doc-private",
				Attributes: map[string]any{"public": true}}}, true},
		{"abac deny missing attributes fail closed", CheckRequest{
			Subject:  subjectOnly("dave"), // no stored subject, no inline attrs
			Action:   "read",
			Resource: CheckResource{Type: "document", Key: "doc-public"}}, false},
		{"abac deny unknown instance without inline attrs", CheckRequest{
			Subject: subjectOnly("carol"), Action: "read",
			Resource: CheckResource{Type: "document", Key: "doc-unknown"}}, false},
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

	decision, err := service.Check(ctx, "other", CheckRequest{
		Subject: subjectOnly("alice"), Action: "write", Resource: CheckResource{Type: "document"}})
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
		{"missing subject", CheckRequest{Action: "read", Resource: CheckResource{Type: "document"}}},
		{"missing action", CheckRequest{Subject: subjectOnly("alice"), Resource: CheckResource{Type: "document"}}},
		{"missing resource type", CheckRequest{Subject: subjectOnly("alice"), Action: "read"}},
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
		{Subject: subjectOnly("alice"), Action: "write", Resource: CheckResource{Type: "document"}},
		{Subject: subjectOnly("bob"), Action: "write", Resource: CheckResource{Type: "document"}},
		{Subject: subjectOnly("carol"), Action: "read", Resource: CheckResource{Type: "document", Key: "doc-public"}},
	})
	require.NoError(t, err)
	require.Len(t, decisions, 3)
	assert.True(t, decisions[0].Allow)
	assert.False(t, decisions[1].Allow)
	assert.True(t, decisions[2].Allow)
}

func TestCheck_ReasonMentionsGrantingRole(t *testing.T) {
	service, ctx := newCheckFixture(t)

	decision, err := service.Check(ctx, "default", CheckRequest{
		Subject: subjectOnly("alice"), Action: "write", Resource: CheckResource{Type: "document"}})
	require.NoError(t, err)
	assert.True(t, decision.Allow)
	assert.Contains(t, decision.Reason, "editor")

	decision, err = service.Check(ctx, "default", CheckRequest{
		Subject: subjectOnly("carol"), Action: "read", Resource: CheckResource{Type: "document", Key: "doc-public"}})
	require.NoError(t, err)
	assert.True(t, decision.Allow)
	assert.Contains(t, decision.Reason, "engineers")
	assert.Contains(t, decision.Reason, "public-docs")
}
