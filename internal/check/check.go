package check

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/latebit-io/az/internal/resources"
	"github.com/latebit-io/az/internal/roles"
)

// CheckRequest asks whether subject may perform action on a resource type.
type CheckRequest struct {
	Subject  string `json:"subject"`
	Action   string `json:"action"`
	Resource string `json:"resource"`
}

// Decision is the check outcome. Denies are decisions, not errors.
type Decision struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason"`
}

type CheckService interface {
	Check(ctx context.Context, tenantID string, request CheckRequest) (Decision, error)
	CheckBulk(ctx context.Context, tenantID string, requests []CheckRequest) ([]Decision, error)
}

type DefaultCheckService struct {
	resourceTypes resources.ResourceTypeRepository
	roles         roles.RoleRepository
	assignments   roles.AssignmentRepository
	logger        *slog.Logger
	decisionLog   bool
}

func NewDefaultCheckService(
	resourceTypes resources.ResourceTypeRepository,
	roleRepo roles.RoleRepository,
	assignments roles.AssignmentRepository,
	logger *slog.Logger,
	decisionLog bool,
) CheckService {
	return &DefaultCheckService{
		resourceTypes: resourceTypes,
		roles:         roleRepo,
		assignments:   assignments,
		logger:        logger,
		decisionLog:   decisionLog,
	}
}

// Check decides whether the subject may perform action on the resource type:
// any assigned role granting resource:action allows. Unknown resource types
// or actions deny rather than error.
func (s *DefaultCheckService) Check(ctx context.Context, tenantID string, request CheckRequest) (Decision, error) {
	started := time.Now()
	decision, err := s.decide(ctx, tenantID, request)
	if err != nil {
		return decision, err
	}
	if s.decisionLog {
		s.logger.Info("decision",
			"tenantId", tenantID,
			"subject", request.Subject,
			"action", request.Action,
			"resource", request.Resource,
			"allow", decision.Allow,
			"reason", decision.Reason,
			"durationMs", time.Since(started).Milliseconds())
	}
	return decision, nil
}

func (s *DefaultCheckService) CheckBulk(ctx context.Context, tenantID string,
	requests []CheckRequest) ([]Decision, error) {
	decisions := make([]Decision, 0, len(requests))
	for _, request := range requests {
		decision, err := s.Check(ctx, tenantID, request)
		if err != nil {
			return decisions, err
		}
		decisions = append(decisions, decision)
	}
	return decisions, nil
}

func (s *DefaultCheckService) decide(ctx context.Context, tenantID string, request CheckRequest) (Decision, error) {
	if request.Subject == "" {
		return Decision{}, InvalidCheckError{Value: "subject is required"}
	}
	if request.Action == "" {
		return Decision{}, InvalidCheckError{Value: "action is required"}
	}
	if request.Resource == "" {
		return Decision{}, InvalidCheckError{Value: "resource is required"}
	}

	// resolve the resource type; unknown type or action is a deny, not an error
	resourceType, err := s.resourceTypes.Read(ctx, tenantID, request.Resource)
	var typeNotFound resources.ResourceTypeNotFoundError
	if errors.As(err, &typeNotFound) {
		return Decision{Allow: false, Reason: fmt.Sprintf("unknown resource type '%s'", request.Resource)}, nil
	}
	if err != nil {
		return Decision{}, err
	}
	if !resourceType.HasAction(request.Action) {
		return Decision{Allow: false, Reason: fmt.Sprintf("unknown action '%s' for resource type '%s'",
			request.Action, request.Resource)}, nil
	}

	roleKeys, err := s.assignments.RolesForSubject(ctx, tenantID, request.Subject)
	if err != nil {
		return Decision{}, err
	}
	if len(roleKeys) > 0 {
		roleKey, granted, err := s.roles.AnyGrants(ctx, tenantID, roleKeys, request.Resource, request.Action)
		if err != nil {
			return Decision{}, err
		}
		if granted {
			return Decision{Allow: true, Reason: fmt.Sprintf("role '%s' grants %s:%s", roleKey,
				request.Resource, request.Action)}, nil
		}
	}

	return Decision{Allow: false, Reason: "no role grants this permission"}, nil
}
