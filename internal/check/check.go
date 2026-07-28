package check

import (
	"context"
	"fmt"
	"log/slog"
	"time"
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
	checks      CheckRepository
	logger      *slog.Logger
	decisionLog bool
}

func NewDefaultCheckService(
	checks CheckRepository,
	logger *slog.Logger,
	decisionLog bool,
) CheckService {
	return &DefaultCheckService{
		checks:      checks,
		logger:      logger,
		decisionLog: decisionLog,
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

	// one round trip resolves type, action and grant together; unknown type
	// or action is a deny, not an error
	grant, err := s.checks.Resolve(ctx, tenantID, request)
	if err != nil {
		return Decision{}, err
	}
	if !grant.TypeFound {
		return Decision{Allow: false, Reason: fmt.Sprintf("unknown resource type '%s'", request.Resource)}, nil
	}
	if !grant.ActionDeclared {
		return Decision{Allow: false, Reason: fmt.Sprintf("unknown action '%s' for resource type '%s'",
			request.Action, request.Resource)}, nil
	}
	if grant.Granted {
		return Decision{Allow: true, Reason: fmt.Sprintf("role '%s' grants %s:%s", grant.RoleName,
			request.Resource, request.Action)}, nil
	}

	return Decision{Allow: false, Reason: "no role grants this permission"}, nil
}
