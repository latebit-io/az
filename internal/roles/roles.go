package roles

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/latebit-io/az/internal/resources"
	"github.com/latebit-io/az/internal/utils"
)

// Permission grants an action on a resource type.
type Permission struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

// Role groups permission grants; subjects get permissions through role
// assignments.
type Role struct {
	ID          string       `json:"id"`
	TenantID    string       `json:"tenantId"`
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	Created     time.Time    `json:"created"`
	Modified    time.Time    `json:"modified"`
}

type RoleService interface {
	Create(ctx context.Context, tenantID, key, name, description string, permissions []Permission) error
	Get(ctx context.Context, tenantID, key string) (*Role, error)
	List(ctx context.Context, tenantID string) ([]Role, error)
	Update(ctx context.Context, tenantID, key, name, description string, permissions []Permission) error
	Delete(ctx context.Context, tenantID, key string) error
}

type DefaultRoleService struct {
	repo      RoleRepository
	resources resources.ResourceService
}

func NewDefaultRoleService(repo RoleRepository, resourceService resources.ResourceService) RoleService {
	return &DefaultRoleService{repo: repo, resources: resourceService}
}

func (s *DefaultRoleService) Create(ctx context.Context, tenantID, key, name, description string,
	permissions []Permission) error {
	if err := s.validateRole(ctx, tenantID, key, name, permissions); err != nil {
		return err
	}
	return s.repo.Create(ctx, Role{
		TenantID:    tenantID,
		Key:         key,
		Name:        name,
		Description: description,
		Permissions: permissions,
	})
}

func (s *DefaultRoleService) Get(ctx context.Context, tenantID, key string) (*Role, error) {
	return s.repo.Read(ctx, tenantID, key)
}

func (s *DefaultRoleService) List(ctx context.Context, tenantID string) ([]Role, error) {
	return s.repo.ReadAll(ctx, tenantID)
}

func (s *DefaultRoleService) Update(ctx context.Context, tenantID, key, name, description string,
	permissions []Permission) error {
	if err := s.validateRole(ctx, tenantID, key, name, permissions); err != nil {
		return err
	}
	return s.repo.Update(ctx, Role{
		TenantID:    tenantID,
		Key:         key,
		Name:        name,
		Description: description,
		Permissions: permissions,
	})
}

func (s *DefaultRoleService) Delete(ctx context.Context, tenantID, key string) error {
	return s.repo.Delete(ctx, tenantID, key)
}

// validateRole checks the role shape and every permission grant against the
// tenant's resource type definitions.
func (s *DefaultRoleService) validateRole(ctx context.Context, tenantID, key, name string,
	permissions []Permission) error {
	if err := utils.ValidateKey(key); err != nil {
		return InvalidRoleError{Value: fmt.Sprintf("key '%s': %s", key, err)}
	}
	if name == "" {
		return InvalidRoleError{Value: "name is required"}
	}

	resourceTypes := map[string]*resources.ResourceType{}
	for _, permission := range permissions {
		resourceType, ok := resourceTypes[permission.Resource]
		if !ok {
			var err error
			resourceType, err = s.resources.Get(ctx, tenantID, permission.Resource)
			var notFound resources.ResourceTypeNotFoundError
			if errors.As(err, &notFound) {
				return InvalidPermissionError{Resource: permission.Resource, Action: permission.Action,
					Reason: "unknown resource type"}
			}
			if err != nil {
				return err
			}
			resourceTypes[permission.Resource] = resourceType
		}
		if !resourceType.HasAction(permission.Action) {
			return InvalidPermissionError{Resource: permission.Resource, Action: permission.Action,
				Reason: "unknown action"}
		}
	}
	return nil
}
