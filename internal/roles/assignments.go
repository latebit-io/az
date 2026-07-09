package roles

import (
	"context"
	"time"

	"github.com/latebit-io/az/internal/utils"
)

// RoleAssignment binds a subject to a role within a tenant. Subjects are
// implicit: an assignment does not require a stored subject document. The
// triple (tenant, subject, role) is the identity — no surrogate id.
type RoleAssignment struct {
	TenantID string    `json:"tenantId"`
	Subject  string    `json:"subject"`
	RoleID   string    `json:"roleId"`
	Created  time.Time `json:"created"`
}

type AssignmentService interface {
	Assign(ctx context.Context, tenantID, subject, roleID string) error
	// List returns assignments filtered by subject and/or role id; empty
	// filters match everything.
	List(ctx context.Context, tenantID, subject, roleID string) ([]RoleAssignment, error)
	Unassign(ctx context.Context, tenantID, subject, roleID string) error
	// RolesForSubject returns the names of the roles assigned to a subject —
	// the shape BulwarkAuth embeds as the JWT roles claim.
	RolesForSubject(ctx context.Context, tenantID, subject string) ([]string, error)
}

type DefaultAssignmentService struct {
	repo AssignmentRepository
}

func NewDefaultAssignmentService(repo AssignmentRepository) AssignmentService {
	return &DefaultAssignmentService{repo: repo}
}

func (s *DefaultAssignmentService) Assign(ctx context.Context, tenantID, subject, roleID string) error {
	if subject == "" {
		return InvalidAssignmentError{Value: "subject is required"}
	}
	if err := utils.ValidateUUID(roleID); err != nil {
		return InvalidAssignmentError{Value: "invalid role id"}
	}
	return s.repo.Create(ctx, RoleAssignment{TenantID: tenantID, Subject: subject, RoleID: roleID})
}

func (s *DefaultAssignmentService) List(ctx context.Context, tenantID, subject, roleID string) ([]RoleAssignment, error) {
	if roleID != "" {
		if err := utils.ValidateUUID(roleID); err != nil {
			return nil, InvalidAssignmentError{Value: "invalid role id"}
		}
	}
	return s.repo.ReadAll(ctx, tenantID, subject, roleID)
}

func (s *DefaultAssignmentService) Unassign(ctx context.Context, tenantID, subject, roleID string) error {
	if err := utils.ValidateUUID(roleID); err != nil {
		return InvalidAssignmentError{Value: "invalid role id"}
	}
	return s.repo.Delete(ctx, tenantID, subject, roleID)
}

func (s *DefaultAssignmentService) RolesForSubject(ctx context.Context, tenantID, subject string) ([]string, error) {
	return s.repo.RolesForSubject(ctx, tenantID, subject)
}
