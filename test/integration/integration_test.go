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

	// resource type
	status, err := az.post("/api/resources", map[string]any{
		"tenantId": tenant, "key": "document",
		"actions": []string{"read", "write", "delete"},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	// role + assignment
	status, err = az.post("/api/roles", map[string]any{
		"tenantId": tenant, "key": "editor", "name": "Editor",
		"permissions": []map[string]string{
			{"resource": "document", "action": "read"},
			{"resource": "document", "action": "write"},
		},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	status, err = az.post("/api/assignments", map[string]any{
		"tenantId": tenant, "subject": "alice@example.com", "role": "editor",
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
		"tenantId": tenant, "key": "document",
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, status)

	// unwind: role (assignments cascade) -> resource type
	status, err = az.put("/api/roles/delete", map[string]any{"tenantId": tenant, "key": "editor"}, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, status)

	status, err = az.put("/api/resources/delete", map[string]any{"tenantId": tenant, "key": "document"}, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, status)
}

func TestTenantIsolation(t *testing.T) {
	tenantA := newTenant()
	tenantB := newTenant()

	status, err := az.post("/api/resources", map[string]any{
		"tenantId": tenantA, "key": "widget", "actions": []string{"use"},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	status, err = az.post("/api/roles", map[string]any{
		"tenantId": tenantA, "key": "user", "name": "User",
		"permissions": []map[string]string{{"resource": "widget", "action": "use"}},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	status, err = az.post("/api/assignments", map[string]any{
		"tenantId": tenantA, "subject": "alice", "role": "user",
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

func TestProblemDetailsShape(t *testing.T) {
	tenant := newTenant()

	var details struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
		Detail string `json:"detail"`
	}
	request := map[string]any{"tenantId": tenant, "key": "missing"}
	response, err := az.rawPost("/api/resources/get", request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusNotFound, response.StatusCode)
	require.NoError(t, jsonDecode(response, &details))
	assert.Equal(t, http.StatusNotFound, details.Status)
	assert.Equal(t, "https://latebit.io/az/errors/", details.Type)
	assert.NotEmpty(t, details.Detail)
}
