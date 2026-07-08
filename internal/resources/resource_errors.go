package resources

import "fmt"

type ResourceTypeDuplicateError struct {
	Value string `json:"value"`
}

func (e ResourceTypeDuplicateError) Error() string {
	return fmt.Sprintf("duplicate resource type: '%s' already exists", e.Value)
}

type ResourceTypeNotFoundError struct {
	Value string `json:"value"`
}

func (e ResourceTypeNotFoundError) Error() string {
	return fmt.Sprintf("resource type not found: %s", e.Value)
}

type ResourceTypeReferencedError struct {
	Value string `json:"value"`
}

func (e ResourceTypeReferencedError) Error() string {
	return fmt.Sprintf("resource type '%s' is referenced and cannot be deleted", e.Value)
}

type UnknownActionError struct {
	ResourceType string `json:"resourceType"`
	Action       string `json:"action"`
}

func (e UnknownActionError) Error() string {
	return fmt.Sprintf("unknown action '%s' for resource type '%s'", e.Action, e.ResourceType)
}

type InvalidResourceTypeError struct {
	Value string `json:"value"`
}

func (e InvalidResourceTypeError) Error() string {
	return fmt.Sprintf("invalid resource type: %s", e.Value)
}

type InstanceDuplicateError struct {
	Value string `json:"value"`
}

func (e InstanceDuplicateError) Error() string {
	return fmt.Sprintf("duplicate resource instance: '%s' already exists", e.Value)
}

type InstanceNotFoundError struct {
	Value string `json:"value"`
}

func (e InstanceNotFoundError) Error() string {
	return fmt.Sprintf("resource instance not found: %s", e.Value)
}

type InvalidInstanceError struct {
	Value string `json:"value"`
}

func (e InvalidInstanceError) Error() string {
	return fmt.Sprintf("invalid resource instance: %s", e.Value)
}
