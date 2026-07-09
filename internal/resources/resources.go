package resources

import (
	"context"
	"fmt"
	"time"

	"github.com/latebit-io/az/internal/utils"
)

// ResourceType defines a protectable resource and the actions that exist on
// it. Permissions are resource:action pairs derived from these definitions.
type ResourceType struct {
	ID       string    `json:"id"`
	TenantID string    `json:"tenantId"`
	Key      string    `json:"key"`
	Actions  []string  `json:"actions"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
}

// HasAction reports whether the resource type declares the action.
func (rt ResourceType) HasAction(action string) bool {
	for _, a := range rt.Actions {
		if a == action {
			return true
		}
	}
	return false
}

type ResourceService interface {
	Create(ctx context.Context, tenantID, key string, actions []string) error
	Get(ctx context.Context, tenantID, key string) (*ResourceType, error)
	List(ctx context.Context, tenantID string) ([]ResourceType, error)
	Update(ctx context.Context, tenantID, key string, actions []string) error
	Delete(ctx context.Context, tenantID, key string) error
}

// ReferenceChecker reports whether something still references a resource
// type; implemented by the roles and conditions repositories and injected in
// main so type deletion can refuse while grants or rules point at it.
type ReferenceChecker interface {
	AnyReferencesResource(ctx context.Context, tenantID, resource string) (bool, error)
}

type DefaultResourceService struct {
	repo     ResourceTypeRepository
	checkers []ReferenceChecker
}

func NewDefaultResourceService(repo ResourceTypeRepository, checkers ...ReferenceChecker) ResourceService {
	return &DefaultResourceService{repo: repo, checkers: checkers}
}

func (s *DefaultResourceService) Create(ctx context.Context, tenantID, key string, actions []string) error {
	if err := validateResourceType(key, actions); err != nil {
		return err
	}
	return s.repo.Create(ctx, ResourceType{
		TenantID: tenantID,
		Key:      key,
		Actions:  actions,
	})
}

func (s *DefaultResourceService) Get(ctx context.Context, tenantID, key string) (*ResourceType, error) {
	return s.repo.Read(ctx, tenantID, key)
}

func (s *DefaultResourceService) List(ctx context.Context, tenantID string) ([]ResourceType, error) {
	return s.repo.ReadAll(ctx, tenantID)
}

func (s *DefaultResourceService) Update(ctx context.Context, tenantID, key string, actions []string) error {
	if err := validateResourceType(key, actions); err != nil {
		return err
	}
	return s.repo.Update(ctx, ResourceType{
		TenantID: tenantID,
		Key:      key,
		Actions:  actions,
	})
}

// Delete removes a resource type and (via FK cascade) its instances. Refuses
// while roles or condition set rules still reference the type.
func (s *DefaultResourceService) Delete(ctx context.Context, tenantID, key string) error {
	for _, checker := range s.checkers {
		referenced, err := checker.AnyReferencesResource(ctx, tenantID, key)
		if err != nil {
			return err
		}
		if referenced {
			return ResourceTypeReferencedError{Value: key}
		}
	}
	return s.repo.Delete(ctx, tenantID, key)
}

func validateResourceType(key string, actions []string) error {
	if err := utils.ValidateKey(key); err != nil {
		return InvalidResourceTypeError{Value: fmt.Sprintf("key '%s': %s", key, err)}
	}
	if len(actions) == 0 {
		return InvalidResourceTypeError{Value: "at least one action is required"}
	}
	for _, action := range actions {
		if err := utils.ValidateKey(action); err != nil {
			return InvalidResourceTypeError{Value: fmt.Sprintf("action '%s': %s", action, err)}
		}
	}
	return nil
}
