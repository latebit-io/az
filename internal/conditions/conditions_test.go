package conditions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func leaf(attribute, operator string, value any) ConditionNode {
	return ConditionNode{Attribute: attribute, Operator: operator, Value: value}
}

func TestConditionNode_EvaluateOperators(t *testing.T) {
	attrs := map[string]any{
		"department": "engineering",
		"level":      float64(5),
		"active":     true,
		"tags":       []any{"admin", "beta"},
		"name":       "fritz seitz",
	}

	tests := []struct {
		name string
		node ConditionNode
		want bool
	}{
		{"equals string true", leaf("department", "equals", "engineering"), true},
		{"equals string false", leaf("department", "equals", "sales"), false},
		{"equals bool true", leaf("active", "equals", true), true},
		{"equals number int vs float", leaf("level", "equals", 5), true},
		{"equals array", leaf("tags", "equals", []any{"admin", "beta"}), true},
		{"not-equals true", leaf("department", "not-equals", "sales"), true},
		{"not-equals false", leaf("department", "not-equals", "engineering"), false},
		{"in true", leaf("department", "in", []any{"sales", "engineering"}), true},
		{"in false", leaf("department", "in", []any{"sales", "marketing"}), false},
		{"in number coercion", leaf("level", "in", []any{1, 5}), true},
		{"not-in true", leaf("department", "not-in", []any{"sales"}), true},
		{"not-in false", leaf("department", "not-in", []any{"engineering"}), false},
		{"gt true", leaf("level", "gt", 4), true},
		{"gt false equal", leaf("level", "gt", 5), false},
		{"gte true equal", leaf("level", "gte", 5), true},
		{"lt true", leaf("level", "lt", 6), true},
		{"lt false", leaf("level", "lt", 5), false},
		{"lte true equal", leaf("level", "lte", 5), true},
		{"contains array true", leaf("tags", "contains", "admin"), true},
		{"contains array false", leaf("tags", "contains", "root"), false},
		{"contains substring true", leaf("name", "contains", "fritz"), true},
		{"contains substring false", leaf("name", "contains", "bob"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.node.Evaluate(attrs))
		})
	}
}

func TestConditionNode_EvaluateFailClosed(t *testing.T) {
	attrs := map[string]any{"present": "value", "nothing": nil}

	// every operator is false when the attribute is missing — including the
	// negated ones
	operators := []struct {
		operator string
		value    any
	}{
		{"equals", "x"},
		{"not-equals", "x"},
		{"in", []any{"x"}},
		{"not-in", []any{"x"}},
		{"gt", 1},
		{"gte", 1},
		{"lt", 1},
		{"lte", 1},
		{"contains", "x"},
	}

	for _, tt := range operators {
		t.Run("missing attribute "+tt.operator, func(t *testing.T) {
			assert.False(t, leaf("missing", tt.operator, tt.value).Evaluate(attrs))
		})
		t.Run("nil attribute "+tt.operator, func(t *testing.T) {
			assert.False(t, leaf("nothing", tt.operator, tt.value).Evaluate(attrs))
		})
	}
}

func TestConditionNode_EvaluateTypeMismatch(t *testing.T) {
	attrs := map[string]any{"department": "engineering", "level": float64(5), "tags": []any{"a"}}

	tests := []struct {
		name string
		node ConditionNode
	}{
		{"gt on string", leaf("department", "gt", 1)},
		{"lt against string value", leaf("level", "lt", "high")},
		{"in with non-array value", leaf("department", "in", "engineering")},
		{"contains on number", leaf("level", "contains", 5)},
		{"equals number vs string", leaf("level", "equals", "5")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, tt.node.Evaluate(attrs))
		})
	}
}

func TestConditionNode_EvaluateNested(t *testing.T) {
	attrs := map[string]any{"department": "engineering", "level": float64(5), "tags": []any{"beta"}}

	node := ConditionNode{AllOf: []ConditionNode{
		leaf("department", "equals", "engineering"),
		{AnyOf: []ConditionNode{
			leaf("level", "gte", 10),
			leaf("tags", "contains", "beta"),
		}},
	}}
	assert.True(t, node.Evaluate(attrs))

	node = ConditionNode{AllOf: []ConditionNode{
		leaf("department", "equals", "engineering"),
		{AnyOf: []ConditionNode{
			leaf("level", "gte", 10),
			leaf("tags", "contains", "admin"),
		}},
	}}
	assert.False(t, node.Evaluate(attrs))

	node = ConditionNode{AnyOf: []ConditionNode{
		leaf("missing", "equals", "x"),
		{AllOf: []ConditionNode{leaf("level", "gt", 1), leaf("level", "lt", 10)}},
	}}
	assert.True(t, node.Evaluate(attrs))
}

func TestConditionNode_Validate(t *testing.T) {
	valid := []struct {
		name string
		node ConditionNode
	}{
		{"leaf", leaf("a", "equals", "x")},
		{"in with array", leaf("a", "in", []any{"x"})},
		{"allOf", ConditionNode{AllOf: []ConditionNode{leaf("a", "equals", "x")}}},
		{"nested", ConditionNode{AnyOf: []ConditionNode{
			{AllOf: []ConditionNode{leaf("a", "gt", 1)}},
			leaf("b", "contains", "x"),
		}}},
	}
	for _, tt := range valid {
		t.Run("valid "+tt.name, func(t *testing.T) {
			assert.NoError(t, tt.node.Validate())
		})
	}

	invalid := []struct {
		name string
		node ConditionNode
	}{
		{"empty node", ConditionNode{}},
		{"empty branch", ConditionNode{AllOf: []ConditionNode{}}},
		{"mixed branch and leaf", ConditionNode{AllOf: []ConditionNode{leaf("a", "equals", "x")},
			Attribute: "a", Operator: "equals", Value: "x"}},
		{"both branches", ConditionNode{AllOf: []ConditionNode{leaf("a", "equals", "x")},
			AnyOf: []ConditionNode{leaf("a", "equals", "x")}}},
		{"unknown operator", leaf("a", "matches", "x")},
		{"no attribute", ConditionNode{Operator: "equals", Value: "x"}},
		{"no value", ConditionNode{Attribute: "a", Operator: "equals"}},
		{"in without array", leaf("a", "in", "x")},
		{"invalid nested child", ConditionNode{AllOf: []ConditionNode{{}}}},
	}
	for _, tt := range invalid {
		t.Run("invalid "+tt.name, func(t *testing.T) {
			assert.Error(t, tt.node.Validate())
		})
	}
}

func TestConditionNode_ValidateMaxDepth(t *testing.T) {
	node := leaf("a", "equals", "x")
	for range 9 {
		node = ConditionNode{AllOf: []ConditionNode{node}}
	}
	assert.NoError(t, node.Validate())

	node = ConditionNode{AllOf: []ConditionNode{node}}
	assert.Error(t, node.Validate())
}

func TestConditionNode_JSONRoundTrip(t *testing.T) {
	raw := `{"allOf":[
		{"attribute":"department","operator":"equals","value":"engineering"},
		{"anyOf":[
			{"attribute":"level","operator":"gte","value":5},
			{"attribute":"tags","operator":"contains","value":"admin"}
		]}
	]}`

	var node ConditionNode
	require.NoError(t, json.Unmarshal([]byte(raw), &node))
	require.NoError(t, node.Validate())

	assert.True(t, node.Evaluate(map[string]any{"department": "engineering", "level": float64(7)}))
	assert.True(t, node.Evaluate(map[string]any{"department": "engineering", "tags": []any{"admin"}}))
	assert.False(t, node.Evaluate(map[string]any{"department": "engineering", "level": float64(1)}))
	assert.False(t, node.Evaluate(map[string]any{"department": "sales", "level": float64(7)}))
}
