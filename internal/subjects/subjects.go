package subjects

import (
	"context"
	"fmt"
	"time"

	"github.com/latebit-io/az/internal/utils"
)

// Subject is a user or service that permissions are checked for. The key is
// opaque — BulwarkAuth accounts conventionally use the account email.
// Subjects need not exist to receive role assignments; a stored subject adds
// attributes for ABAC condition sets.
type Subject struct {
	ID         string         `json:"id"`
	TenantID   string         `json:"tenantId"`
	Key        string         `json:"key"`
	Email      string         `json:"email,omitempty"`
	Attributes map[string]any `json:"attributes"`
	Created    time.Time      `json:"created"`
	Modified   time.Time      `json:"modified"`
}

type SubjectService interface {
	Create(ctx context.Context, tenantID, key, email string, attributes map[string]any) error
	Get(ctx context.Context, tenantID, key string) (*Subject, error)
	List(ctx context.Context, tenantID string) ([]Subject, error)
	Update(ctx context.Context, tenantID, key, email string, attributes map[string]any) error
	Delete(ctx context.Context, tenantID, key string) error
}

// AssignmentCleaner removes a subject's role assignments; implemented by the
// roles package's assignment repository and injected in main to keep subject
// deletion cascading without a package cycle.
type AssignmentCleaner interface {
	DeleteAllForSubject(ctx context.Context, tenantID, subject string) error
}

type DefaultSubjectService struct {
	repo        SubjectRepository
	assignments AssignmentCleaner
	txManager   utils.TxManager
}

func NewDefaultSubjectService(repo SubjectRepository, assignments AssignmentCleaner,
	txManager utils.TxManager) SubjectService {
	return &DefaultSubjectService{repo: repo, assignments: assignments, txManager: txManager}
}

func (s *DefaultSubjectService) Create(ctx context.Context, tenantID, key, email string, attributes map[string]any) error {
	if err := validateSubject(key, email); err != nil {
		return err
	}
	return s.repo.Create(ctx, Subject{
		TenantID:   tenantID,
		Key:        key,
		Email:      email,
		Attributes: attributes,
	})
}

func (s *DefaultSubjectService) Get(ctx context.Context, tenantID, key string) (*Subject, error) {
	return s.repo.Read(ctx, tenantID, key)
}

func (s *DefaultSubjectService) List(ctx context.Context, tenantID string) ([]Subject, error) {
	return s.repo.ReadAll(ctx, tenantID)
}

func (s *DefaultSubjectService) Update(ctx context.Context, tenantID, key, email string, attributes map[string]any) error {
	if err := validateSubject(key, email); err != nil {
		return err
	}
	return s.repo.Update(ctx, Subject{
		TenantID:   tenantID,
		Key:        key,
		Email:      email,
		Attributes: attributes,
	})
}

// Delete removes the subject and its role assignments in one transaction.
func (s *DefaultSubjectService) Delete(ctx context.Context, tenantID, key string) error {
	return s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.assignments.DeleteAllForSubject(txCtx, tenantID, key); err != nil {
			return err
		}
		return s.repo.Delete(txCtx, tenantID, key)
	})
}

func validateSubject(key, email string) error {
	if key == "" {
		return InvalidSubjectError{Value: "key is required"}
	}
	if len(key) > 254 {
		return InvalidSubjectError{Value: "key too long"}
	}
	if email != "" {
		if err := utils.ValidateEmail(email); err != nil {
			return InvalidSubjectError{Value: fmt.Sprintf("email '%s': %s", email, err)}
		}
	}
	return nil
}
