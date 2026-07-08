package roles

import (
	"context"
	"time"
)

// RoleAssignment binds a subject to a role within a tenant. Subjects are
// implicit (permit.io style): an assignment does not require a stored
// subject document.
type RoleAssignment struct {
	ID       string    `json:"id"`
	TenantID string    `json:"tenantId"`
	Subject  string    `json:"subject"`
	Role     string    `json:"role"`
	Created  time.Time `json:"created"`
}

type AssignmentService interface {
	Assign(ctx context.Context, tenantID, subject, role string) error
	// List returns assignments filtered by subject and/or role; empty filters
	// match everything.
	List(ctx context.Context, tenantID, subject, role string) ([]RoleAssignment, error)
	Unassign(ctx context.Context, tenantID, subject, role string) error
	// RolesForSubject returns the role keys assigned to a subject — the shape
	// BulwarkAuth embeds as the JWT roles claim.
	RolesForSubject(ctx context.Context, tenantID, subject string) ([]string, error)
}

type DefaultAssignmentService struct {
	repo AssignmentRepository
}

func NewDefaultAssignmentService(repo AssignmentRepository) AssignmentService {
	return &DefaultAssignmentService{repo: repo}
}

func (s *DefaultAssignmentService) Assign(ctx context.Context, tenantID, subject, role string) error {
	if subject == "" {
		return InvalidAssignmentError{Value: "subject is required"}
	}
	if role == "" {
		return InvalidAssignmentError{Value: "role is required"}
	}
	return s.repo.Create(ctx, RoleAssignment{TenantID: tenantID, Subject: subject, Role: role})
}

func (s *DefaultAssignmentService) List(ctx context.Context, tenantID, subject, role string) ([]RoleAssignment, error) {
	return s.repo.ReadAll(ctx, tenantID, subject, role)
}

func (s *DefaultAssignmentService) Unassign(ctx context.Context, tenantID, subject, role string) error {
	return s.repo.Delete(ctx, tenantID, subject, role)
}

func (s *DefaultAssignmentService) RolesForSubject(ctx context.Context, tenantID, subject string) ([]string, error) {
	return s.repo.RolesForSubject(ctx, tenantID, subject)
}
