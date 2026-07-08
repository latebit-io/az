package conditions

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Operators supported by condition leaves.
const (
	OperatorEquals    = "equals"
	OperatorNotEquals = "not-equals"
	OperatorIn        = "in"
	OperatorNotIn     = "not-in"
	OperatorGt        = "gt"
	OperatorGte       = "gte"
	OperatorLt        = "lt"
	OperatorLte       = "lte"
	OperatorContains  = "contains"
)

// maxDepth bounds condition tree nesting.
const maxDepth = 10

// ConditionNode is a recursive condition tree. A node is exactly one of:
// an allOf branch, an anyOf branch, or a leaf (attribute + operator + value).
//
// Evaluation is fail-closed: a leaf whose attribute is missing evaluates
// false for every operator, including not-equals and not-in.
type ConditionNode struct {
	AllOf     []ConditionNode `json:"allOf,omitempty"`
	AnyOf     []ConditionNode `json:"anyOf,omitempty"`
	Attribute string          `json:"attribute,omitempty"`
	Operator  string          `json:"operator,omitempty"`
	Value     any             `json:"value,omitempty"`
}

// Validate checks the tree structurally: exactly one form per node, known
// operators, array values for in/not-in, non-empty branches, bounded depth.
func (n ConditionNode) Validate() error {
	return n.validate(1)
}

func (n ConditionNode) validate(depth int) error {
	if depth > maxDepth {
		return fmt.Errorf("condition tree exceeds max depth of %d", maxDepth)
	}

	forms := 0
	if n.AllOf != nil {
		forms++
	}
	if n.AnyOf != nil {
		forms++
	}
	if n.Attribute != "" || n.Operator != "" || n.Value != nil {
		forms++
		if n.AllOf != nil || n.AnyOf != nil {
			return errors.New("condition node cannot mix a branch with leaf fields")
		}
	}
	if forms == 0 {
		return errors.New("condition node must be allOf, anyOf or a leaf")
	}
	if forms > 1 {
		return errors.New("condition node must be exactly one of allOf, anyOf or a leaf")
	}

	if n.AllOf != nil || n.AnyOf != nil {
		branch := n.AllOf
		if n.AnyOf != nil {
			branch = n.AnyOf
		}
		if len(branch) == 0 {
			return errors.New("condition branch must not be empty")
		}
		for _, child := range branch {
			if err := child.validate(depth + 1); err != nil {
				return err
			}
		}
		return nil
	}

	if n.Attribute == "" {
		return errors.New("condition leaf requires an attribute")
	}
	switch n.Operator {
	case OperatorEquals, OperatorNotEquals, OperatorGt, OperatorGte, OperatorLt, OperatorLte, OperatorContains:
	case OperatorIn, OperatorNotIn:
		if _, ok := n.Value.([]any); !ok {
			return fmt.Errorf("operator %s requires an array value", n.Operator)
		}
	default:
		return fmt.Errorf("unknown operator '%s'", n.Operator)
	}
	if n.Value == nil {
		return errors.New("condition leaf requires a value")
	}
	return nil
}

// Evaluate reports whether the attributes satisfy the condition tree.
// Branches short-circuit; a missing attribute makes any leaf false.
func (n ConditionNode) Evaluate(attrs map[string]any) bool {
	if n.AllOf != nil {
		for _, child := range n.AllOf {
			if !child.Evaluate(attrs) {
				return false
			}
		}
		return true
	}
	if n.AnyOf != nil {
		for _, child := range n.AnyOf {
			if child.Evaluate(attrs) {
				return true
			}
		}
		return false
	}

	attribute, ok := attrs[n.Attribute]
	if !ok || attribute == nil {
		return false
	}

	switch n.Operator {
	case OperatorEquals:
		return jsonEqual(attribute, n.Value)
	case OperatorNotEquals:
		return !jsonEqual(attribute, n.Value)
	case OperatorIn:
		return jsonContains(n.Value, attribute)
	case OperatorNotIn:
		values, ok := n.Value.([]any)
		if !ok {
			return false
		}
		for _, value := range values {
			if jsonEqual(attribute, value) {
				return false
			}
		}
		return true
	case OperatorGt, OperatorGte, OperatorLt, OperatorLte:
		left, leftOk := toFloat(attribute)
		right, rightOk := toFloat(n.Value)
		if !leftOk || !rightOk {
			return false
		}
		switch n.Operator {
		case OperatorGt:
			return left > right
		case OperatorGte:
			return left >= right
		case OperatorLt:
			return left < right
		default:
			return left <= right
		}
	case OperatorContains:
		if jsonContains(attribute, n.Value) {
			return true
		}
		attrString, attrIsString := attribute.(string)
		valueString, valueIsString := n.Value.(string)
		return attrIsString && valueIsString && strings.Contains(attrString, valueString)
	default:
		return false
	}
}

// jsonEqual compares two values through JSON normalization so numeric types
// (int vs float64) and nested structures compare consistently.
func jsonEqual(a, b any) bool {
	if left, leftOk := toFloat(a); leftOk {
		if right, rightOk := toFloat(b); rightOk {
			return left == right
		}
		return false
	}
	aJSON, errA := json.Marshal(a)
	bJSON, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(aJSON) == string(bJSON)
}

// jsonContains reports whether collection is an array containing value.
func jsonContains(collection, value any) bool {
	values, ok := collection.([]any)
	if !ok {
		return false
	}
	for _, item := range values {
		if jsonEqual(item, value) {
			return true
		}
	}
	return false
}

func toFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
