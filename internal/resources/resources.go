package resources

import (
	"context"
	"fmt"
	"time"

	"github.com/latebit-io/az/internal/utils"
)

// ResourceType defines a protectable resource and the actions that exist on
// it. Permissions are resource:action pairs derived from these definitions;
// role grants reference them by foreign key.
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

type DefaultResourceService struct {
	repo      ResourceTypeRepository
	txManager utils.TxManager
}

func NewDefaultResourceService(repo ResourceTypeRepository, txManager utils.TxManager) ResourceService {
	return &DefaultResourceService{repo: repo, txManager: txManager}
}

func (s *DefaultResourceService) Create(ctx context.Context, tenantID, key string, actions []string) error {
	actions, err := validateResourceType(key, actions)
	if err != nil {
		return err
	}
	return s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.repo.Create(txCtx, ResourceType{
			TenantID: tenantID,
			Key:      key,
			Actions:  actions,
		})
	})
}

func (s *DefaultResourceService) Get(ctx context.Context, tenantID, key string) (*ResourceType, error) {
	return s.repo.Read(ctx, tenantID, key)
}

func (s *DefaultResourceService) List(ctx context.Context, tenantID string) ([]ResourceType, error) {
	return s.repo.ReadAll(ctx, tenantID)
}

// Update replaces the declared actions. Removing an action that a role still
// grants is refused (foreign key RESTRICT).
func (s *DefaultResourceService) Update(ctx context.Context, tenantID, key string, actions []string) error {
	actions, err := validateResourceType(key, actions)
	if err != nil {
		return err
	}
	return s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.repo.Update(txCtx, ResourceType{
			TenantID: tenantID,
			Key:      key,
			Actions:  actions,
		})
	})
}

// Delete removes a resource type and its actions (FK cascade). Refused while
// any role permission still references an action (FK RESTRICT).
func (s *DefaultResourceService) Delete(ctx context.Context, tenantID, key string) error {
	return s.repo.Delete(ctx, tenantID, key)
}

// validateResourceType checks keys and returns the deduplicated action list.
func validateResourceType(key string, actions []string) ([]string, error) {
	if err := utils.ValidateKey(key); err != nil {
		return nil, InvalidResourceTypeError{Value: fmt.Sprintf("key '%s': %s", key, err)}
	}
	if len(actions) == 0 {
		return nil, InvalidResourceTypeError{Value: "at least one action is required"}
	}
	seen := make(map[string]bool, len(actions))
	deduped := make([]string, 0, len(actions))
	for _, action := range actions {
		if err := utils.ValidateKey(action); err != nil {
			return nil, InvalidResourceTypeError{Value: fmt.Sprintf("action '%s': %s", action, err)}
		}
		if !seen[action] {
			seen[action] = true
			deduped = append(deduped, action)
		}
	}
	return deduped, nil
}
