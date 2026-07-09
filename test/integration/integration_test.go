package integration

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	baseURI = "http://localhost:8080"
	az      *client
)

func TestMain(m *testing.M) {
	if uri := os.Getenv("AZ_BASE_URI"); uri != "" {
		baseURI = uri
	}
	az = newClient(baseURI)
	az.apiKey = os.Getenv("API_KEY")

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

type decision struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason"`
}

// newTenant returns a unique tenant id so runs are isolated and repeatable
// against the same database.
func newTenant() string {
	return "t" + uuid.NewString()[:12]
}

func TestFullAuthorizationFlow(t *testing.T) {
	tenant := newTenant()

	// resource type; create returns the type with its generated id
	var document struct {
		ID string `json:"id"`
	}
	status, err := az.post("/api/resources", map[string]any{
		"tenantId": tenant, "name": "document",
		"actions": []string{"read", "write", "delete"},
	}, &document)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)
	require.NotEmpty(t, document.ID)

	// role + assignment; create returns the role with its generated id
	var editor struct {
		ID string `json:"id"`
	}
	status, err = az.post("/api/roles", map[string]any{
		"tenantId": tenant, "name": "editor",
		"permissions": []map[string]string{
			{"resource": "document", "action": "read"},
			{"resource": "document", "action": "write"},
		},
	}, &editor)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)
	require.NotEmpty(t, editor.ID)

	status, err = az.post("/api/assignments", map[string]any{
		"tenantId": tenant, "subject": "alice@example.com", "roleId": editor.ID,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	// roles-for-subject (the JWT claim payload)
	var rolesResponse struct {
		Roles []string `json:"roles"`
	}
	status, err = az.post("/api/subjects/roles", map[string]any{
		"tenantId": tenant, "key": "alice@example.com",
	}, &rolesResponse)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, []string{"editor"}, rolesResponse.Roles)

	// checks
	var allowDecision decision
	status, err = az.post("/api/check", map[string]any{
		"tenantId": tenant, "subject": "alice@example.com", "action": "write", "resource": "document",
	}, &allowDecision)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	assert.True(t, allowDecision.Allow, allowDecision.Reason)

	var denyDecision decision
	status, err = az.post("/api/check", map[string]any{
		"tenantId": tenant, "subject": "alice@example.com", "action": "delete", "resource": "document",
	}, &denyDecision)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	assert.False(t, denyDecision.Allow, denyDecision.Reason)

	// bulk
	var bulk struct {
		Results []decision `json:"results"`
	}
	status, err = az.post("/api/check/bulk", map[string]any{
		"tenantId": tenant,
		"checks": []map[string]any{
			{"subject": "alice@example.com", "action": "write", "resource": "document"},
			{"subject": "alice@example.com", "action": "delete", "resource": "document"},
		},
	}, &bulk)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, bulk.Results, 2)
	assert.True(t, bulk.Results[0].Allow)
	assert.False(t, bulk.Results[1].Allow)

	// referenced resource type cannot be deleted
	status, err = az.put("/api/resources/delete", map[string]any{
		"tenantId": tenant, "id": document.ID,
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, status)

	// unwind: role (grants + assignments cascade) -> resource type
	status, err = az.put("/api/roles/delete", map[string]any{"tenantId": tenant, "id": editor.ID}, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, status)

	status, err = az.put("/api/resources/delete", map[string]any{"tenantId": tenant, "id": document.ID}, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, status)
}

func TestTenantIsolation(t *testing.T) {
	tenantA := newTenant()
	tenantB := newTenant()

	status, err := az.post("/api/resources", map[string]any{
		"tenantId": tenantA, "name": "widget", "actions": []string{"use"},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	var user struct {
		ID string `json:"id"`
	}
	status, err = az.post("/api/roles", map[string]any{
		"tenantId": tenantA, "name": "User",
		"permissions": []map[string]string{{"resource": "widget", "action": "use"}},
	}, &user)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	status, err = az.post("/api/assignments", map[string]any{
		"tenantId": tenantA, "subject": "alice", "roleId": user.ID,
	}, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	// allowed in tenant A
	var allowDecision decision
	status, err = az.post("/api/check", map[string]any{
		"tenantId": tenantA, "subject": "alice", "action": "use", "resource": "widget",
	}, &allowDecision)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	assert.True(t, allowDecision.Allow)

	// denied in tenant B — the resource type does not even exist there
	var denyDecision decision
	status, err = az.post("/api/check", map[string]any{
		"tenantId": tenantB, "subject": "alice", "action": "use", "resource": "widget",
	}, &denyDecision)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	assert.False(t, denyDecision.Allow)
}

// TestApiKeyTenantScoping needs the service running with BOOTSTRAP_API_KEY
// set and API_KEY exported for the suite; skipped otherwise.
func TestApiKeyTenantScoping(t *testing.T) {
	if az.apiKey == "" {
		t.Skip("API_KEY not set — service running without api key auth")
	}
	tenant := newTenant()

	// no key → 401
	bare := newClient(baseURI)
	status, err := bare.post("/api/roles/list", map[string]any{"tenantId": tenant}, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, status)

	// bootstrap key mints a tenant-scoped key
	var created struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	status, err = az.post("/api/apikeys", map[string]any{"tenantId": tenant, "name": "ci"}, &created)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)
	require.NotEmpty(t, created.Key)

	// seed policy in the tenant with the bootstrap key
	status, err = az.post("/api/resources", map[string]any{
		"tenantId": tenant, "name": "widget", "actions": []string{"use"},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	// the tenant key works within its tenant
	tenantClient := newClient(baseURI)
	tenantClient.apiKey = created.Key

	var types []map[string]any
	status, err = tenantClient.post("/api/resources/list", map[string]any{}, &types)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	assert.Len(t, types, 1)

	// a tenant key cannot escape its tenant: asking for another tenant's
	// data still returns its own
	status, err = tenantClient.post("/api/resources/list", map[string]any{"tenantId": "default"}, &types)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	assert.Len(t, types, 1)
	assert.Equal(t, tenant, types[0]["tenantId"])

	// a tenant key cannot manage api keys
	status, err = tenantClient.post("/api/apikeys", map[string]any{"name": "sneaky"}, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, status)

	// revoked keys stop working
	status, err = az.put("/api/apikeys/delete", map[string]any{"tenantId": tenant, "id": created.ID}, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, status)

	status, err = tenantClient.post("/api/resources/list", map[string]any{}, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestProblemDetailsShape(t *testing.T) {
	tenant := newTenant()

	var details struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
		Detail string `json:"detail"`
	}
	request := map[string]any{"tenantId": tenant, "id": "00000000-0000-0000-0000-000000000000"}
	response, err := az.rawPost("/api/resources/get", request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusNotFound, response.StatusCode)
	require.NoError(t, jsonDecode(response, &details))
	assert.Equal(t, http.StatusNotFound, details.Status)
	assert.Equal(t, "https://latebit.io/az/errors/", details.Type)
	assert.NotEmpty(t, details.Detail)
}
