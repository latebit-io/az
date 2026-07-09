package roles

import (
	"context"
	"fmt"
	"time"

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
	txManager utils.TxManager
}

func NewDefaultRoleService(repo RoleRepository, txManager utils.TxManager) RoleService {
	return &DefaultRoleService{repo: repo, txManager: txManager}
}

// Create stores the role and its permission grants in one transaction. A
// grant referencing an undeclared resource:action fails the foreign key and
// surfaces as InvalidPermissionError.
func (s *DefaultRoleService) Create(ctx context.Context, tenantID, key, name, description string,
	permissions []Permission) error {
	if err := validateRole(key, name); err != nil {
		return err
	}
	return s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.repo.Create(txCtx, Role{
			TenantID:    tenantID,
			Key:         key,
			Name:        name,
			Description: description,
			Permissions: permissions,
		})
	})
}

func (s *DefaultRoleService) Get(ctx context.Context, tenantID, key string) (*Role, error) {
	return s.repo.Read(ctx, tenantID, key)
}

func (s *DefaultRoleService) List(ctx context.Context, tenantID string) ([]Role, error) {
	return s.repo.ReadAll(ctx, tenantID)
}

// Update replaces the role's name, description and permission grants in one
// transaction; grants are validated by foreign key.
func (s *DefaultRoleService) Update(ctx context.Context, tenantID, key, name, description string,
	permissions []Permission) error {
	if err := validateRole(key, name); err != nil {
		return err
	}
	return s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.repo.Update(txCtx, Role{
			TenantID:    tenantID,
			Key:         key,
			Name:        name,
			Description: description,
			Permissions: permissions,
		})
	})
}

// Delete removes the role; its permission grants and assignments cascade.
func (s *DefaultRoleService) Delete(ctx context.Context, tenantID, key string) error {
	return s.repo.Delete(ctx, tenantID, key)
}

func validateRole(key, name string) error {
	if err := utils.ValidateKey(key); err != nil {
		return InvalidRoleError{Value: fmt.Sprintf("key '%s': %s", key, err)}
	}
	if name == "" {
		return InvalidRoleError{Value: "name is required"}
	}
	return nil
}
