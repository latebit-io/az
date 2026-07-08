package conditions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/latebit-io/az/internal/resources"
)

// ConditionSetRule grants a permission (resource:action) to every subject in
// a subject set on every resource in a resource set.
type ConditionSetRule struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenantId"`
	SubjectSet  string    `json:"subjectSet"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
	ResourceSet string    `json:"resourceSet"`
	Created     time.Time `json:"created"`
}

func (r ConditionSetRule) String() string {
	return fmt.Sprintf("%s -> %s:%s -> %s", r.SubjectSet, r.Resource, r.Action, r.ResourceSet)
}

type RuleService interface {
	Create(ctx context.Context, tenantID, subjectSet, resource, action, resourceSet string) error
	List(ctx context.Context, tenantID string) ([]ConditionSetRule, error)
	Delete(ctx context.Context, tenantID, subjectSet, resource, action, resourceSet string) error
}

type DefaultRuleService struct {
	repo      RuleRepository
	sets      ConditionSetRepository
	resources resources.ResourceService
}

func NewDefaultRuleService(repo RuleRepository, sets ConditionSetRepository,
	resourceService resources.ResourceService) RuleService {
	return &DefaultRuleService{repo: repo, sets: sets, resources: resourceService}
}

func (s *DefaultRuleService) Create(ctx context.Context, tenantID, subjectSet, resource, action,
	resourceSet string) error {
	rule := ConditionSetRule{
		TenantID:    tenantID,
		SubjectSet:  subjectSet,
		Resource:    resource,
		Action:      action,
		ResourceSet: resourceSet,
	}
	if err := s.validateRule(ctx, rule); err != nil {
		return err
	}
	return s.repo.Create(ctx, rule)
}

func (s *DefaultRuleService) List(ctx context.Context, tenantID string) ([]ConditionSetRule, error) {
	return s.repo.ReadAll(ctx, tenantID)
}

func (s *DefaultRuleService) Delete(ctx context.Context, tenantID, subjectSet, resource, action,
	resourceSet string) error {
	return s.repo.Delete(ctx, ConditionSetRule{
		TenantID:    tenantID,
		SubjectSet:  subjectSet,
		Resource:    resource,
		Action:      action,
		ResourceSet: resourceSet,
	})
}

// validateRule checks that both sets exist with the right types, the
// permission exists, and the resource set is bound to the rule's resource.
func (s *DefaultRuleService) validateRule(ctx context.Context, rule ConditionSetRule) error {
	resourceType, err := s.resources.Get(ctx, rule.TenantID, rule.Resource)
	var typeNotFound resources.ResourceTypeNotFoundError
	if errors.As(err, &typeNotFound) {
		return InvalidRuleError{Value: fmt.Sprintf("unknown resource type '%s'", rule.Resource)}
	}
	if err != nil {
		return err
	}
	if !resourceType.HasAction(rule.Action) {
		return InvalidRuleError{Value: fmt.Sprintf("unknown action '%s' for resource type '%s'",
			rule.Action, rule.Resource)}
	}

	subjectSet, err := s.sets.Read(ctx, rule.TenantID, rule.SubjectSet)
	var setNotFound ConditionSetNotFoundError
	if errors.As(err, &setNotFound) {
		return InvalidRuleError{Value: fmt.Sprintf("unknown subject set '%s'", rule.SubjectSet)}
	}
	if err != nil {
		return err
	}
	if subjectSet.Type != SetTypeSubject {
		return InvalidRuleError{Value: fmt.Sprintf("set '%s' is not a subject set", rule.SubjectSet)}
	}

	resourceSet, err := s.sets.Read(ctx, rule.TenantID, rule.ResourceSet)
	if errors.As(err, &setNotFound) {
		return InvalidRuleError{Value: fmt.Sprintf("unknown resource set '%s'", rule.ResourceSet)}
	}
	if err != nil {
		return err
	}
	if resourceSet.Type != SetTypeResource {
		return InvalidRuleError{Value: fmt.Sprintf("set '%s' is not a resource set", rule.ResourceSet)}
	}
	if resourceSet.ResourceType != rule.Resource {
		return InvalidRuleError{Value: fmt.Sprintf("resource set '%s' is bound to resource type '%s', not '%s'",
			rule.ResourceSet, resourceSet.ResourceType, rule.Resource)}
	}
	return nil
}
