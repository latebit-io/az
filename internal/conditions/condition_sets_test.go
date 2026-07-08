package conditions

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/latebit-io/az/internal/resources"
	"github.com/latebit-io/az/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	os.Exit(utils.RunTestMain(m))
}

func newConditionFixture(t *testing.T) (*pgxpool.Pool, ConditionSetService, RuleService, context.Context) {
	pool := utils.NewTestPool(t)
	resourceService := resources.NewDefaultResourceService(resources.NewPostgresResourceTypeRepository(pool))
	setRepo := NewPostgresConditionSetRepository(pool)
	setService := NewDefaultConditionSetService(setRepo, resourceService)
	ruleService := NewDefaultRuleService(NewPostgresRuleRepository(pool), setRepo, resourceService)
	ctx := context.Background()
	require.NoError(t, resourceService.Create(ctx, "default", "document", "Document", "",
		[]string{"read", "write"}, nil))
	return pool, setService, ruleService, ctx
}

func subjectSet(key string) ConditionSet {
	return ConditionSet{
		TenantID: "default",
		Key:      key,
		Name:     key,
		Type:     SetTypeSubject,
		Conditions: ConditionNode{
			Attribute: "department", Operator: OperatorEquals, Value: "engineering",
		},
	}
}

func resourceSet(key, resourceType string) ConditionSet {
	return ConditionSet{
		TenantID:     "default",
		Key:          key,
		Name:         key,
		Type:         SetTypeResource,
		ResourceType: resourceType,
		Conditions: ConditionNode{
			Attribute: "public", Operator: OperatorEquals, Value: true,
		},
	}
}

func TestConditionSetService_CreateAndGet(t *testing.T) {
	_, setService, _, ctx := newConditionFixture(t)

	require.NoError(t, setService.Create(ctx, subjectSet("engineers")))
	require.NoError(t, setService.Create(ctx, resourceSet("public-docs", "document")))

	set, err := setService.Get(ctx, "default", "engineers")
	require.NoError(t, err)
	assert.Equal(t, SetTypeSubject, set.Type)
	assert.Equal(t, "department", set.Conditions.Attribute)

	set, err = setService.Get(ctx, "default", "public-docs")
	require.NoError(t, err)
	assert.Equal(t, SetTypeResource, set.Type)
	assert.Equal(t, "document", set.ResourceType)
}

func TestConditionSetService_Validation(t *testing.T) {
	_, setService, _, ctx := newConditionFixture(t)

	tests := []struct {
		name string
		set  ConditionSet
	}{
		{"bad key", ConditionSet{TenantID: "default", Key: "Bad Key", Name: "x", Type: SetTypeSubject,
			Conditions: ConditionNode{Attribute: "a", Operator: "equals", Value: "x"}}},
		{"no name", ConditionSet{TenantID: "default", Key: "set-1", Type: SetTypeSubject,
			Conditions: ConditionNode{Attribute: "a", Operator: "equals", Value: "x"}}},
		{"bad type", ConditionSet{TenantID: "default", Key: "set-1", Name: "x", Type: "group",
			Conditions: ConditionNode{Attribute: "a", Operator: "equals", Value: "x"}}},
		{"subject set with resource type", func() ConditionSet {
			set := subjectSet("set-1")
			set.ResourceType = "document"
			return set
		}()},
		{"resource set without resource type", func() ConditionSet {
			set := resourceSet("set-1", "document")
			set.ResourceType = ""
			return set
		}()},
		{"resource set with unknown resource type", resourceSet("set-1", "missing")},
		{"invalid conditions", ConditionSet{TenantID: "default", Key: "set-1", Name: "x", Type: SetTypeSubject,
			Conditions: ConditionNode{Attribute: "a", Operator: "matches", Value: "x"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setService.Create(ctx, tt.set)
			var invalid InvalidConditionSetError
			assert.ErrorAs(t, err, &invalid)
		})
	}
}

func TestConditionSetService_DuplicateUpdateDelete(t *testing.T) {
	_, setService, _, ctx := newConditionFixture(t)

	require.NoError(t, setService.Create(ctx, subjectSet("engineers")))

	err := setService.Create(ctx, subjectSet("engineers"))
	var duplicate ConditionSetDuplicateError
	assert.ErrorAs(t, err, &duplicate)

	updated := subjectSet("engineers")
	updated.Conditions = ConditionNode{Attribute: "level", Operator: OperatorGte, Value: 5}
	require.NoError(t, setService.Update(ctx, updated))
	set, err := setService.Get(ctx, "default", "engineers")
	require.NoError(t, err)
	assert.Equal(t, "level", set.Conditions.Attribute)

	require.NoError(t, setService.Delete(ctx, "default", "engineers"))
	_, err = setService.Get(ctx, "default", "engineers")
	var notFound ConditionSetNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestRuleService_CreateListDelete(t *testing.T) {
	_, setService, ruleService, ctx := newConditionFixture(t)

	require.NoError(t, setService.Create(ctx, subjectSet("engineers")))
	require.NoError(t, setService.Create(ctx, resourceSet("public-docs", "document")))

	require.NoError(t, ruleService.Create(ctx, "default", "engineers", "document", "read", "public-docs"))

	rules, err := ruleService.List(ctx, "default")
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, "engineers", rules[0].SubjectSet)
	assert.Equal(t, "public-docs", rules[0].ResourceSet)

	err = ruleService.Create(ctx, "default", "engineers", "document", "read", "public-docs")
	var duplicate RuleDuplicateError
	assert.ErrorAs(t, err, &duplicate)

	require.NoError(t, ruleService.Delete(ctx, "default", "engineers", "document", "read", "public-docs"))
	err = ruleService.Delete(ctx, "default", "engineers", "document", "read", "public-docs")
	var notFound RuleNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestRuleService_Validation(t *testing.T) {
	_, setService, ruleService, ctx := newConditionFixture(t)

	require.NoError(t, setService.Create(ctx, subjectSet("engineers")))
	require.NoError(t, setService.Create(ctx, resourceSet("public-docs", "document")))

	tests := []struct {
		name       string
		subjectSet string
		resource   string
		action     string
		resSet     string
	}{
		{"unknown resource", "engineers", "missing", "read", "public-docs"},
		{"unknown action", "engineers", "document", "share", "public-docs"},
		{"unknown subject set", "missing", "document", "read", "public-docs"},
		{"unknown resource set", "engineers", "document", "read", "missing"},
		{"subject set as resource set", "engineers", "document", "read", "engineers"},
		{"resource set as subject set", "public-docs", "document", "read", "public-docs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ruleService.Create(ctx, "default", tt.subjectSet, tt.resource, tt.action, tt.resSet)
			var invalid InvalidRuleError
			assert.ErrorAs(t, err, &invalid)
		})
	}
}

func TestConditionSetService_DeleteReferencedByRule(t *testing.T) {
	_, setService, ruleService, ctx := newConditionFixture(t)

	require.NoError(t, setService.Create(ctx, subjectSet("engineers")))
	require.NoError(t, setService.Create(ctx, resourceSet("public-docs", "document")))
	require.NoError(t, ruleService.Create(ctx, "default", "engineers", "document", "read", "public-docs"))

	err := setService.Delete(ctx, "default", "engineers")
	var referenced ConditionSetReferencedError
	assert.ErrorAs(t, err, &referenced)

	require.NoError(t, ruleService.Delete(ctx, "default", "engineers", "document", "read", "public-docs"))
	assert.NoError(t, setService.Delete(ctx, "default", "engineers"))
}

func TestResourceTypeDeleteBlockedByReferences(t *testing.T) {
	pool := utils.NewTestPool(t)
	roleRepoStub := NewPostgresRuleRepository(pool)
	setRepo := NewPostgresConditionSetRepository(pool)
	resourceService := resources.NewDefaultResourceService(resources.NewPostgresResourceTypeRepository(pool),
		setRepo, roleRepoStub)
	setService := NewDefaultConditionSetService(setRepo, resourceService)
	ctx := context.Background()

	require.NoError(t, resourceService.Create(ctx, "default", "document", "Document", "", []string{"read"}, nil))
	require.NoError(t, setService.Create(ctx, ConditionSet{
		TenantID: "default", Key: "public-docs", Name: "public docs", Type: SetTypeResource,
		ResourceType: "document",
		Conditions:   ConditionNode{Attribute: "public", Operator: OperatorEquals, Value: true},
	}))

	err := resourceService.Delete(ctx, "default", "document")
	var referenced resources.ResourceTypeReferencedError
	assert.ErrorAs(t, err, &referenced)

	require.NoError(t, setService.Delete(ctx, "default", "public-docs"))
	assert.NoError(t, resourceService.Delete(ctx, "default", "document"))
}
