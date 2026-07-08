package resources

import (
	"context"
	"fmt"
	"time"

	"github.com/latebit-io/az/internal/utils"
)

// ResourceInstance is a concrete resource of a resource type, identified by
// key, carrying attributes used by ABAC condition sets at check time.
type ResourceInstance struct {
	ID           string         `json:"id"`
	TenantID     string         `json:"tenantId"`
	ResourceType string         `json:"resourceType"`
	Key          string         `json:"key"`
	Attributes   map[string]any `json:"attributes"`
	Created      time.Time      `json:"created"`
	Modified     time.Time      `json:"modified"`
}

type InstanceService interface {
	Create(ctx context.Context, tenantID, resourceType, key string, attributes map[string]any) error
	Get(ctx context.Context, tenantID, resourceType, key string) (*ResourceInstance, error)
	List(ctx context.Context, tenantID, resourceType string) ([]ResourceInstance, error)
	Update(ctx context.Context, tenantID, resourceType, key string, attributes map[string]any) error
	Delete(ctx context.Context, tenantID, resourceType, key string) error
}

type DefaultInstanceService struct {
	repo InstanceRepository
}

func NewDefaultInstanceService(repo InstanceRepository) InstanceService {
	return &DefaultInstanceService{repo: repo}
}

func (s *DefaultInstanceService) Create(ctx context.Context, tenantID, resourceType, key string,
	attributes map[string]any) error {
	if err := validateInstance(resourceType, key); err != nil {
		return err
	}
	return s.repo.Create(ctx, ResourceInstance{
		TenantID:     tenantID,
		ResourceType: resourceType,
		Key:          key,
		Attributes:   attributes,
	})
}

func (s *DefaultInstanceService) Get(ctx context.Context, tenantID, resourceType, key string) (*ResourceInstance, error) {
	return s.repo.Read(ctx, tenantID, resourceType, key)
}

func (s *DefaultInstanceService) List(ctx context.Context, tenantID, resourceType string) ([]ResourceInstance, error) {
	return s.repo.ReadAll(ctx, tenantID, resourceType)
}

func (s *DefaultInstanceService) Update(ctx context.Context, tenantID, resourceType, key string,
	attributes map[string]any) error {
	if err := validateInstance(resourceType, key); err != nil {
		return err
	}
	return s.repo.Update(ctx, ResourceInstance{
		TenantID:     tenantID,
		ResourceType: resourceType,
		Key:          key,
		Attributes:   attributes,
	})
}

func (s *DefaultInstanceService) Delete(ctx context.Context, tenantID, resourceType, key string) error {
	return s.repo.Delete(ctx, tenantID, resourceType, key)
}

func validateInstance(resourceType, key string) error {
	if err := utils.ValidateKey(resourceType); err != nil {
		return InvalidInstanceError{Value: fmt.Sprintf("resource type '%s': %s", resourceType, err)}
	}
	if err := utils.ValidateKey(key); err != nil {
		return InvalidInstanceError{Value: fmt.Sprintf("key '%s': %s", key, err)}
	}
	return nil
}
