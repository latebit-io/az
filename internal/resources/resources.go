package resources

import (
	"context"
	"fmt"
	"time"

	"github.com/latebit-io/az/internal/utils"
)

// ResourceType defines a protectable resource and the actions that exist on
// it. Permissions are resource:action pairs derived from these definitions;
// role grants reference them by foreign key. Resource types are addressed by
// their uuid id; name is the unique per-tenant handle used in permission
// grants and check requests.
type ResourceType struct {
	ID       string    `json:"id"`
	TenantID string    `json:"tenantId"`
	Name     string    `json:"name"`
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
	Create(ctx context.Context, tenantID, name string, actions []string) (*ResourceType, error)
	Get(ctx context.Context, tenantID, id string) (*ResourceType, error)
	List(ctx context.Context, tenantID string) ([]ResourceType, error)
	Update(ctx context.Context, tenantID, id, name string, actions []string) error
	Delete(ctx context.Context, tenantID, id string) error
}

type DefaultResourceService struct {
	repo      ResourceTypeRepository
	txManager utils.TxManager
}

func NewDefaultResourceService(repo ResourceTypeRepository, txManager utils.TxManager) ResourceService {
	return &DefaultResourceService{repo: repo, txManager: txManager}
}

// Create stores the resource type and its actions in one transaction and
// returns the type with its generated id.
func (s *DefaultResourceService) Create(ctx context.Context, tenantID, name string,
	actions []string) (*ResourceType, error) {
	actions, err := validateResourceType(name, actions)
	if err != nil {
		return nil, err
	}
	var created *ResourceType
	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		var err error
		created, err = s.repo.Create(txCtx, ResourceType{
			TenantID: tenantID,
			Name:     name,
			Actions:  actions,
		})
		return err
	})
	return created, err
}

func (s *DefaultResourceService) Get(ctx context.Context, tenantID, id string) (*ResourceType, error) {
	if err := utils.ValidateUUID(id); err != nil {
		return nil, InvalidResourceTypeError{Value: "invalid resource type id"}
	}
	return s.repo.Read(ctx, tenantID, id)
}

func (s *DefaultResourceService) List(ctx context.Context, tenantID string) ([]ResourceType, error) {
	return s.repo.ReadAll(ctx, tenantID)
}

// Update replaces the name and the declared actions. Removing an action that
// a role still grants is refused (foreign key RESTRICT).
func (s *DefaultResourceService) Update(ctx context.Context, tenantID, id, name string, actions []string) error {
	if err := utils.ValidateUUID(id); err != nil {
		return InvalidResourceTypeError{Value: "invalid resource type id"}
	}
	actions, err := validateResourceType(name, actions)
	if err != nil {
		return err
	}
	return s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.repo.Update(txCtx, ResourceType{
			ID:       id,
			TenantID: tenantID,
			Name:     name,
			Actions:  actions,
		})
	})
}

// Delete removes a resource type and its actions (FK cascade). Refused while
// any role permission still references an action (FK RESTRICT).
func (s *DefaultResourceService) Delete(ctx context.Context, tenantID, id string) error {
	if err := utils.ValidateUUID(id); err != nil {
		return InvalidResourceTypeError{Value: "invalid resource type id"}
	}
	return s.repo.Delete(ctx, tenantID, id)
}

// validateResourceType checks names and returns the deduplicated action list.
func validateResourceType(name string, actions []string) ([]string, error) {
	if err := utils.ValidateKey(name); err != nil {
		return nil, InvalidResourceTypeError{Value: fmt.Sprintf("name '%s': %s", name, err)}
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
