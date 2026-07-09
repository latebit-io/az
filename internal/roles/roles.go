package roles

import (
	"context"
	"time"

	"github.com/latebit-io/az/internal/utils"
)

// Permission grants an action on a resource type.
type Permission struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

// Role groups permission grants; subjects get permissions through role
// assignments. Roles are addressed by their uuid id; name is the unique
// per-tenant label (and what the JWT roles claim carries).
type Role struct {
	ID          string       `json:"id"`
	TenantID    string       `json:"tenantId"`
	Name        string       `json:"name"`
	Permissions []Permission `json:"permissions"`
	Created     time.Time    `json:"created"`
	Modified    time.Time    `json:"modified"`
}

type RoleService interface {
	Create(ctx context.Context, tenantID, name string, permissions []Permission) (*Role, error)
	Get(ctx context.Context, tenantID, id string) (*Role, error)
	List(ctx context.Context, tenantID string) ([]Role, error)
	Update(ctx context.Context, tenantID, id, name string, permissions []Permission) error
	Delete(ctx context.Context, tenantID, id string) error
}

type DefaultRoleService struct {
	repo      RoleRepository
	txManager utils.TxManager
}

func NewDefaultRoleService(repo RoleRepository, txManager utils.TxManager) RoleService {
	return &DefaultRoleService{repo: repo, txManager: txManager}
}

// Create stores the role and its permission grants in one transaction and
// returns the role with its generated id. A grant referencing an undeclared
// resource:action fails the foreign key and surfaces as
// InvalidPermissionError.
func (s *DefaultRoleService) Create(ctx context.Context, tenantID, name string,
	permissions []Permission) (*Role, error) {
	if name == "" {
		return nil, InvalidRoleError{Value: "name is required"}
	}
	var created *Role
	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		var err error
		created, err = s.repo.Create(txCtx, Role{
			TenantID:    tenantID,
			Name:        name,
			Permissions: permissions,
		})
		return err
	})
	return created, err
}

func (s *DefaultRoleService) Get(ctx context.Context, tenantID, id string) (*Role, error) {
	if err := utils.ValidateUUID(id); err != nil {
		return nil, InvalidRoleError{Value: "invalid role id"}
	}
	return s.repo.Read(ctx, tenantID, id)
}

func (s *DefaultRoleService) List(ctx context.Context, tenantID string) ([]Role, error) {
	return s.repo.ReadAll(ctx, tenantID)
}

// Update replaces the role's name and permission grants in one transaction;
// grants are validated by foreign key.
func (s *DefaultRoleService) Update(ctx context.Context, tenantID, id, name string,
	permissions []Permission) error {
	if err := utils.ValidateUUID(id); err != nil {
		return InvalidRoleError{Value: "invalid role id"}
	}
	if name == "" {
		return InvalidRoleError{Value: "name is required"}
	}
	return s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.repo.Update(txCtx, Role{
			ID:          id,
			TenantID:    tenantID,
			Name:        name,
			Permissions: permissions,
		})
	})
}

// Delete removes the role; its permission grants and assignments cascade.
func (s *DefaultRoleService) Delete(ctx context.Context, tenantID, id string) error {
	if err := utils.ValidateUUID(id); err != nil {
		return InvalidRoleError{Value: "invalid role id"}
	}
	return s.repo.Delete(ctx, tenantID, id)
}
