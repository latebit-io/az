package conditions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/latebit-io/az/internal/resources"
	"github.com/latebit-io/az/internal/utils"
)

// Condition set types.
const (
	SetTypeSubject  = "subject"
	SetTypeResource = "resource"
)

// ConditionSet is a named group of subjects or resources defined by attribute
// conditions (permit.io user-sets / resource-sets). Resource sets are bound
// to one resource type.
type ConditionSet struct {
	ID           string        `json:"id"`
	TenantID     string        `json:"tenantId"`
	Key          string        `json:"key"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Type         string        `json:"type"`
	ResourceType string        `json:"resourceType,omitempty"`
	Conditions   ConditionNode `json:"conditions"`
	Created      time.Time     `json:"created"`
	Modified     time.Time     `json:"modified"`
}

type ConditionSetService interface {
	Create(ctx context.Context, set ConditionSet) error
	Get(ctx context.Context, tenantID, key string) (*ConditionSet, error)
	List(ctx context.Context, tenantID, setType string) ([]ConditionSet, error)
	Update(ctx context.Context, set ConditionSet) error
	Delete(ctx context.Context, tenantID, key string) error
}

type DefaultConditionSetService struct {
	repo      ConditionSetRepository
	resources resources.ResourceService
}

func NewDefaultConditionSetService(repo ConditionSetRepository,
	resourceService resources.ResourceService) ConditionSetService {
	return &DefaultConditionSetService{repo: repo, resources: resourceService}
}

func (s *DefaultConditionSetService) Create(ctx context.Context, set ConditionSet) error {
	if err := s.validateSet(ctx, &set); err != nil {
		return err
	}
	return s.repo.Create(ctx, set)
}

func (s *DefaultConditionSetService) Get(ctx context.Context, tenantID, key string) (*ConditionSet, error) {
	return s.repo.Read(ctx, tenantID, key)
}

func (s *DefaultConditionSetService) List(ctx context.Context, tenantID, setType string) ([]ConditionSet, error) {
	return s.repo.ReadAll(ctx, tenantID, setType)
}

func (s *DefaultConditionSetService) Update(ctx context.Context, set ConditionSet) error {
	if err := s.validateSet(ctx, &set); err != nil {
		return err
	}
	return s.repo.Update(ctx, set)
}

func (s *DefaultConditionSetService) Delete(ctx context.Context, tenantID, key string) error {
	return s.repo.Delete(ctx, tenantID, key)
}

func (s *DefaultConditionSetService) validateSet(ctx context.Context, set *ConditionSet) error {
	if err := utils.ValidateKey(set.Key); err != nil {
		return InvalidConditionSetError{Value: fmt.Sprintf("key '%s': %s", set.Key, err)}
	}
	if set.Name == "" {
		return InvalidConditionSetError{Value: "name is required"}
	}
	switch set.Type {
	case SetTypeSubject:
		if set.ResourceType != "" {
			return InvalidConditionSetError{Value: "resourceType is only valid on resource sets"}
		}
	case SetTypeResource:
		if set.ResourceType == "" {
			return InvalidConditionSetError{Value: "resourceType is required on resource sets"}
		}
		_, err := s.resources.Get(ctx, set.TenantID, set.ResourceType)
		var notFound resources.ResourceTypeNotFoundError
		if errors.As(err, &notFound) {
			return InvalidConditionSetError{Value: fmt.Sprintf("unknown resource type '%s'", set.ResourceType)}
		}
		if err != nil {
			return err
		}
	default:
		return InvalidConditionSetError{Value: "type must be 'subject' or 'resource'"}
	}
	if err := set.Conditions.Validate(); err != nil {
		return InvalidConditionSetError{Value: fmt.Sprintf("conditions: %s", err)}
	}
	return nil
}
