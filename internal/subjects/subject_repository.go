package subjects

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/latebit-io/az/internal/utils"
)

const pgUniqueViolation = "23505"

type SubjectRepository interface {
	Create(ctx context.Context, subject Subject) error
	Read(ctx context.Context, tenantID, key string) (*Subject, error)
	ReadAll(ctx context.Context, tenantID string) ([]Subject, error)
	Update(ctx context.Context, subject Subject) error
	Delete(ctx context.Context, tenantID, key string) error
}

type PostgresSubjectRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresSubjectRepository(pool *pgxpool.Pool) SubjectRepository {
	return &PostgresSubjectRepository{pool: pool}
}

func (r *PostgresSubjectRepository) Create(ctx context.Context, subject Subject) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	attributes, err := marshalAttributes(subject.Attributes)
	if err != nil {
		return err
	}
	_, err = querier.Exec(ctx,
		"INSERT INTO subjects (tenant_id, key, email, attributes) VALUES ($1, $2, $3, $4)",
		subject.TenantID, subject.Key, subject.Email, attributes)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return SubjectDuplicateError{Value: subject.Key}
	}
	return err
}

func (r *PostgresSubjectRepository) Read(ctx context.Context, tenantID, key string) (*Subject, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	row := querier.QueryRow(ctx,
		"SELECT id, tenant_id, key, email, attributes, created, modified FROM subjects WHERE tenant_id = $1 AND key = $2",
		tenantID, key)
	subject, err := scanSubject(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, SubjectNotFoundError{Value: key}
	}
	if err != nil {
		return nil, err
	}
	return subject, nil
}

func (r *PostgresSubjectRepository) ReadAll(ctx context.Context, tenantID string) ([]Subject, error) {
	querier := utils.QuerierFrom(ctx, r.pool)
	rows, err := querier.Query(ctx,
		"SELECT id, tenant_id, key, email, attributes, created, modified FROM subjects WHERE tenant_id = $1 ORDER BY key",
		tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subjectList []Subject
	for rows.Next() {
		subject, err := scanSubject(rows)
		if err != nil {
			return subjectList, err
		}
		subjectList = append(subjectList, *subject)
	}
	return subjectList, rows.Err()
}

func (r *PostgresSubjectRepository) Update(ctx context.Context, subject Subject) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	attributes, err := marshalAttributes(subject.Attributes)
	if err != nil {
		return err
	}
	tag, err := querier.Exec(ctx,
		"UPDATE subjects SET email = $3, attributes = $4, modified = now() WHERE tenant_id = $1 AND key = $2",
		subject.TenantID, subject.Key, subject.Email, attributes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return SubjectNotFoundError{Value: subject.Key}
	}
	return nil
}

func (r *PostgresSubjectRepository) Delete(ctx context.Context, tenantID, key string) error {
	querier := utils.QuerierFrom(ctx, r.pool)
	tag, err := querier.Exec(ctx, "DELETE FROM subjects WHERE tenant_id = $1 AND key = $2", tenantID, key)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return SubjectNotFoundError{Value: key}
	}
	return nil
}

func marshalAttributes(attributes map[string]any) ([]byte, error) {
	if attributes == nil {
		attributes = map[string]any{}
	}
	return json.Marshal(attributes)
}

func scanSubject(row pgx.Row) (*Subject, error) {
	var subject Subject
	var attributes []byte
	err := row.Scan(&subject.ID, &subject.TenantID, &subject.Key, &subject.Email, &attributes,
		&subject.Created, &subject.Modified)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(attributes, &subject.Attributes); err != nil {
		return nil, err
	}
	return &subject, nil
}
