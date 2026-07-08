package conditions

import "fmt"

type ConditionSetDuplicateError struct {
	Value string `json:"value"`
}

func (e ConditionSetDuplicateError) Error() string {
	return fmt.Sprintf("duplicate condition set: '%s' already exists", e.Value)
}

type ConditionSetNotFoundError struct {
	Value string `json:"value"`
}

func (e ConditionSetNotFoundError) Error() string {
	return fmt.Sprintf("condition set not found: %s", e.Value)
}

type ConditionSetReferencedError struct {
	Value string `json:"value"`
}

func (e ConditionSetReferencedError) Error() string {
	return fmt.Sprintf("condition set '%s' is referenced by a rule and cannot be deleted", e.Value)
}

type InvalidConditionSetError struct {
	Value string `json:"value"`
}

func (e InvalidConditionSetError) Error() string {
	return fmt.Sprintf("invalid condition set: %s", e.Value)
}

type RuleDuplicateError struct {
	Value string `json:"value"`
}

func (e RuleDuplicateError) Error() string {
	return fmt.Sprintf("duplicate rule: %s", e.Value)
}

type RuleNotFoundError struct {
	Value string `json:"value"`
}

func (e RuleNotFoundError) Error() string {
	return fmt.Sprintf("rule not found: %s", e.Value)
}

type InvalidRuleError struct {
	Value string `json:"value"`
}

func (e InvalidRuleError) Error() string {
	return fmt.Sprintf("invalid rule: %s", e.Value)
}
