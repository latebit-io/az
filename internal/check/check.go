package check

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/latebit-io/az/internal/conditions"
	"github.com/latebit-io/az/internal/resources"
	"github.com/latebit-io/az/internal/roles"
	"github.com/latebit-io/az/internal/subjects"
)

// CheckSubject identifies who is asking. Inline attributes are merged over
// the stored subject's attributes (inline wins per key).
type CheckSubject struct {
	Key        string         `json:"key"`
	Attributes map[string]any `json:"attributes"`
}

// CheckResource identifies what is being accessed. Key is optional; when set,
// the stored instance's attributes are merged under the inline ones.
type CheckResource struct {
	Type       string         `json:"type"`
	Key        string         `json:"key"`
	Attributes map[string]any `json:"attributes"`
}

type CheckRequest struct {
	Subject  CheckSubject  `json:"subject"`
	Action   string        `json:"action"`
	Resource CheckResource `json:"resource"`
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
	instances     resources.InstanceRepository
	subjects      subjects.SubjectRepository
	roles         roles.RoleRepository
	assignments   roles.AssignmentRepository
	sets          conditions.ConditionSetRepository
	rules         conditions.RuleRepository
	logger        *slog.Logger
	decisionLog   bool
}

func NewDefaultCheckService(
	resourceTypes resources.ResourceTypeRepository,
	instances resources.InstanceRepository,
	subjectRepo subjects.SubjectRepository,
	roleRepo roles.RoleRepository,
	assignments roles.AssignmentRepository,
	sets conditions.ConditionSetRepository,
	rules conditions.RuleRepository,
	logger *slog.Logger,
	decisionLog bool,
) CheckService {
	return &DefaultCheckService{
		resourceTypes: resourceTypes,
		instances:     instances,
		subjects:      subjectRepo,
		roles:         roleRepo,
		assignments:   assignments,
		sets:          sets,
		rules:         rules,
		logger:        logger,
		decisionLog:   decisionLog,
	}
}

// Check decides whether the subject may perform action on the resource:
// RBAC first (role assignments joined against permission grants), then ABAC
// (condition set rules evaluated over merged attributes). Unknown resource
// types or actions deny rather than error.
func (s *DefaultCheckService) Check(ctx context.Context, tenantID string, request CheckRequest) (Decision, error) {
	started := time.Now()
	decision, err := s.decide(ctx, tenantID, request)
	if err != nil {
		return decision, err
	}
	if s.decisionLog {
		s.logger.Info("decision",
			"tenantId", tenantID,
			"subject", request.Subject.Key,
			"action", request.Action,
			"resourceType", request.Resource.Type,
			"resourceKey", request.Resource.Key,
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
	if request.Subject.Key == "" {
		return Decision{}, InvalidCheckError{Value: "subject.key is required"}
	}
	if request.Action == "" {
		return Decision{}, InvalidCheckError{Value: "action is required"}
	}
	if request.Resource.Type == "" {
		return Decision{}, InvalidCheckError{Value: "resource.type is required"}
	}

	// resolve the resource type; unknown type or action is a deny, not an error
	resourceType, err := s.resourceTypes.Read(ctx, tenantID, request.Resource.Type)
	var typeNotFound resources.ResourceTypeNotFoundError
	if errors.As(err, &typeNotFound) {
		return Decision{Allow: false, Reason: fmt.Sprintf("unknown resource type '%s'", request.Resource.Type)}, nil
	}
	if err != nil {
		return Decision{}, err
	}
	if !resourceType.HasAction(request.Action) {
		return Decision{Allow: false, Reason: fmt.Sprintf("unknown action '%s' for resource type '%s'",
			request.Action, request.Resource.Type)}, nil
	}

	// RBAC: any assigned role granting resource:action allows
	roleKeys, err := s.assignments.RolesForSubject(ctx, tenantID, request.Subject.Key)
	if err != nil {
		return Decision{}, err
	}
	if len(roleKeys) > 0 {
		roleKey, granted, err := s.roles.AnyGrants(ctx, tenantID, roleKeys, request.Resource.Type, request.Action)
		if err != nil {
			return Decision{}, err
		}
		if granted {
			return Decision{Allow: true, Reason: fmt.Sprintf("role '%s' grants %s:%s", roleKey,
				request.Resource.Type, request.Action)}, nil
		}
	}

	// ABAC: condition set rules for this permission
	rules, err := s.rules.ReadForPermission(ctx, tenantID, request.Resource.Type, request.Action)
	if err != nil {
		return Decision{}, err
	}
	if len(rules) == 0 {
		return Decision{Allow: false, Reason: "no matching role or condition set rule"}, nil
	}

	subjectAttrs, err := s.subjectAttributes(ctx, tenantID, request.Subject)
	if err != nil {
		return Decision{}, err
	}
	resourceAttrs, err := s.resourceAttributes(ctx, tenantID, request.Resource)
	if err != nil {
		return Decision{}, err
	}

	setKeys := make([]string, 0, len(rules)*2)
	for _, rule := range rules {
		setKeys = append(setKeys, rule.SubjectSet, rule.ResourceSet)
	}
	sets, err := s.sets.ReadMany(ctx, tenantID, setKeys)
	if err != nil {
		return Decision{}, err
	}

	for _, rule := range rules {
		subjectSet, ok := sets[rule.SubjectSet]
		if !ok {
			s.logger.Warn("rule references missing subject set", "tenantId", tenantID, "rule", rule.String())
			continue
		}
		resourceSet, ok := sets[rule.ResourceSet]
		if !ok {
			s.logger.Warn("rule references missing resource set", "tenantId", tenantID, "rule", rule.String())
			continue
		}
		if resourceSet.ResourceType != request.Resource.Type {
			continue
		}
		if subjectSet.Conditions.Evaluate(subjectAttrs) && resourceSet.Conditions.Evaluate(resourceAttrs) {
			return Decision{Allow: true, Reason: fmt.Sprintf("condition rule %s", rule.String())}, nil
		}
	}

	return Decision{Allow: false, Reason: "no matching role or condition set rule"}, nil
}

// subjectAttributes merges the stored subject's attributes (if any) with the
// inline ones; inline wins per key.
func (s *DefaultCheckService) subjectAttributes(ctx context.Context, tenantID string,
	subject CheckSubject) (map[string]any, error) {
	stored, err := s.subjects.Read(ctx, tenantID, subject.Key)
	var notFound subjects.SubjectNotFoundError
	if err != nil && !errors.As(err, &notFound) {
		return nil, err
	}
	var storedAttrs map[string]any
	if stored != nil {
		storedAttrs = stored.Attributes
	}
	return mergeAttributes(storedAttrs, subject.Attributes), nil
}

// resourceAttributes merges the stored instance's attributes (when a resource
// key is given) with the inline ones; inline wins per key.
func (s *DefaultCheckService) resourceAttributes(ctx context.Context, tenantID string,
	resource CheckResource) (map[string]any, error) {
	var storedAttrs map[string]any
	if resource.Key != "" {
		stored, err := s.instances.Read(ctx, tenantID, resource.Type, resource.Key)
		var notFound resources.InstanceNotFoundError
		if err != nil && !errors.As(err, &notFound) {
			return nil, err
		}
		if stored != nil {
			storedAttrs = stored.Attributes
		}
	}
	return mergeAttributes(storedAttrs, resource.Attributes), nil
}

// mergeAttributes shallow-merges inline over stored (top-level keys only).
func mergeAttributes(stored, inline map[string]any) map[string]any {
	merged := make(map[string]any, len(stored)+len(inline))
	for key, value := range stored {
		merged[key] = value
	}
	for key, value := range inline {
		merged[key] = value
	}
	return merged
}
