package roles

import "fmt"

type RoleDuplicateError struct {
	Value string `json:"value"`
}

func (e RoleDuplicateError) Error() string {
	return fmt.Sprintf("duplicate role: '%s' already exists", e.Value)
}

type RoleNotFoundError struct {
	Value string `json:"value"`
}

func (e RoleNotFoundError) Error() string {
	return fmt.Sprintf("role not found: %s", e.Value)
}

type InvalidRoleError struct {
	Value string `json:"value"`
}

func (e InvalidRoleError) Error() string {
	return fmt.Sprintf("invalid role: %s", e.Value)
}

type InvalidPermissionError struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

func (e InvalidPermissionError) Error() string {
	return fmt.Sprintf("invalid permission '%s:%s': %s", e.Resource, e.Action, e.Reason)
}

type AssignmentDuplicateError struct {
	Subject string `json:"subject"`
	Role    string `json:"role"`
}

func (e AssignmentDuplicateError) Error() string {
	return fmt.Sprintf("assignment of role '%s' to subject '%s' already exists", e.Role, e.Subject)
}

type AssignmentNotFoundError struct {
	Subject string `json:"subject"`
	Role    string `json:"role"`
}

func (e AssignmentNotFoundError) Error() string {
	return fmt.Sprintf("assignment of role '%s' to subject '%s' not found", e.Role, e.Subject)
}

type InvalidAssignmentError struct {
	Value string `json:"value"`
}

func (e InvalidAssignmentError) Error() string {
	return fmt.Sprintf("invalid assignment: %s", e.Value)
}
