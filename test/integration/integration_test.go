package integration

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	az "github.com/latebit-io/az-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	baseURI = "http://localhost:8080"
	apiKey  string
	client  *az.Client
)

func TestMain(m *testing.M) {
	if uri := os.Getenv("AZ_BASE_URI"); uri != "" {
		baseURI = uri
	}
	apiKey = os.Getenv("API_KEY")
	client = az.NewClient(baseURI, apiKey, nil)

	// wait for the service to be reachable
	ready := false
	for range 30 {
		response, err := http.Get(baseURI + "/health")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(time.Second)
	}
	if !ready {
		fmt.Fprintln(os.Stderr, "az service not reachable on "+baseURI+" — start it with docker-compose up")
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// newTenant returns a unique tenant id so runs are isolated and repeatable
// against the same database.
func newTenant() string {
	return "t" + uuid.NewString()[:12]
}

func TestFullAuthorizationFlow(t *testing.T) {
	ctx := context.Background()
	tenant := newTenant()

	// resource type; create returns the type with its generated id
	document, err := client.Resources.Create(ctx, tenant, "document", []string{"read", "write", "delete"})
	require.NoError(t, err)
	require.NotEmpty(t, document.ID)

	// role + assignment; create returns the role with its generated id
	editor, err := client.Roles.Create(ctx, tenant, "editor", []az.Permission{
		{Resource: "document", Action: "read"},
		{Resource: "document", Action: "write"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, editor.ID)

	require.NoError(t, client.Assignments.Assign(ctx, tenant, "alice@example.com", editor.ID))

	// roles-for-subject (the JWT claim payload)
	roles, err := client.Assignments.SubjectRoles(ctx, tenant, "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"editor"}, roles)

	// checks — denies are decisions, not errors
	allowDecision, err := client.Check(ctx, tenant, "alice@example.com", "write", "document")
	require.NoError(t, err)
	assert.True(t, allowDecision.Allow, allowDecision.Reason)

	denyDecision, err := client.Check(ctx, tenant, "alice@example.com", "delete", "document")
	require.NoError(t, err)
	assert.False(t, denyDecision.Allow, denyDecision.Reason)

	// bulk
	results, err := client.CheckBulk(ctx, tenant, []az.CheckRequest{
		{Subject: "alice@example.com", Action: "write", Resource: "document"},
		{Subject: "alice@example.com", Action: "delete", Resource: "document"},
	})
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.True(t, results[0].Allow)
	assert.False(t, results[1].Allow)

	// referenced resource type cannot be deleted
	err = client.Resources.Delete(ctx, tenant, document.ID)
	problem := requireProblem(t, err)
	assert.Equal(t, http.StatusConflict, problem.Status)

	// unwind: role (grants + assignments cascade) -> resource type
	require.NoError(t, client.Roles.Delete(ctx, tenant, editor.ID))
	require.NoError(t, client.Resources.Delete(ctx, tenant, document.ID))
}

func TestTenantIsolation(t *testing.T) {
	ctx := context.Background()
	tenantA := newTenant()
	tenantB := newTenant()

	_, err := client.Resources.Create(ctx, tenantA, "widget", []string{"use"})
	require.NoError(t, err)

	user, err := client.Roles.Create(ctx, tenantA, "User",
		[]az.Permission{{Resource: "widget", Action: "use"}})
	require.NoError(t, err)

	require.NoError(t, client.Assignments.Assign(ctx, tenantA, "alice", user.ID))

	// allowed in tenant A
	allowDecision, err := client.Check(ctx, tenantA, "alice", "use", "widget")
	require.NoError(t, err)
	assert.True(t, allowDecision.Allow)

	// denied in tenant B — the resource type does not even exist there
	denyDecision, err := client.Check(ctx, tenantB, "alice", "use", "widget")
	require.NoError(t, err)
	assert.False(t, denyDecision.Allow)
}

// TestApiKeyTenantScoping needs the service running with BOOTSTRAP_API_KEY
// set and API_KEY exported for the suite; skipped otherwise.
func TestApiKeyTenantScoping(t *testing.T) {
	if apiKey == "" {
		t.Skip("API_KEY not set — service running without api key auth")
	}
	ctx := context.Background()
	tenant := newTenant()

	// no key → 401
	_, err := az.NewClient(baseURI, "", nil).Roles.List(ctx, tenant)
	problem := requireProblem(t, err)
	assert.Equal(t, http.StatusUnauthorized, problem.Status)

	// bootstrap key mints a tenant-scoped key
	created, err := client.ApiKeys.Create(ctx, tenant, "ci")
	require.NoError(t, err)
	require.NotEmpty(t, created.Key)

	// seed policy in the tenant with the bootstrap key, and a decoy in
	// default so a cross-tenant leak would actually surface below
	_, err = client.Resources.Create(ctx, tenant, "widget", []string{"use"})
	require.NoError(t, err)
	decoy := "decoy-" + newTenant()
	_, err = client.Resources.Create(ctx, "default", decoy, []string{"use"})
	require.NoError(t, err)

	// the tenant key works within its tenant and sees only its own data
	tenantClient := az.NewClient(baseURI, created.Key, nil)

	types, err := tenantClient.Resources.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, types, 1)
	assert.Equal(t, tenant, types[0].TenantID)

	// a tenant key cannot escape its tenant: asking for another tenant's
	// data still returns its own, not default's
	types, err = tenantClient.Resources.List(ctx, "default")
	require.NoError(t, err)
	require.Len(t, types, 1)
	assert.Equal(t, tenant, types[0].TenantID)

	// a tenant key cannot manage api keys
	_, err = tenantClient.ApiKeys.Create(ctx, "", "sneaky")
	problem = requireProblem(t, err)
	assert.Equal(t, http.StatusForbidden, problem.Status)

	// revoked keys stop working
	require.NoError(t, client.ApiKeys.Revoke(ctx, tenant, created.ID))

	_, err = tenantClient.Resources.List(ctx, "")
	problem = requireProblem(t, err)
	assert.Equal(t, http.StatusUnauthorized, problem.Status)
}

func TestProblemDetailsShape(t *testing.T) {
	ctx := context.Background()
	tenant := newTenant()

	_, err := client.Resources.Get(ctx, tenant, "00000000-0000-0000-0000-000000000000")
	problem := requireProblem(t, err)
	assert.Equal(t, http.StatusNotFound, problem.Status)
	assert.Equal(t, "https://latebit.io/az/errors/", problem.Type)
	assert.NotEmpty(t, problem.Detail)
}

func requireProblem(t *testing.T, err error) *az.Error {
	t.Helper()
	require.Error(t, err)
	var problem *az.Error
	require.True(t, errors.As(err, &problem), "expected problem details, got: %v", err)
	return problem
}
