package subjects

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/latebit-io/az/api/problem"
	"github.com/latebit-io/az/internal/roles"
	"github.com/latebit-io/az/internal/subjects"
	"github.com/latebit-io/az/internal/utils"
)

type SubjectHandler struct {
	subjects    subjects.SubjectService
	assignments roles.AssignmentService
}

type SubjectRequest struct {
	TenantID   string         `json:"tenantId"`
	Key        string         `json:"key"`
	Email      string         `json:"email"`
	Attributes map[string]any `json:"attributes"`
}

type GetSubjectRequest struct {
	TenantID string `json:"tenantId"`
	Key      string `json:"key"`
}

type ListSubjectsRequest struct {
	TenantID string `json:"tenantId"`
}

type SubjectRolesResponse struct {
	Roles []string `json:"roles"`
}

func NewSubjectHandler(service subjects.SubjectService, assignments roles.AssignmentService) SubjectHandler {
	return SubjectHandler{subjects: service, assignments: assignments}
}

// Create registers a subject with attributes for ABAC checks.
func (sh SubjectHandler) Create(c *echo.Context) error {
	request := new(SubjectRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := sh.subjects.Create(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Key,
		request.Email, request.Attributes)
	if err != nil {
		return subjectProblem(c, err)
	}

	return c.NoContent(http.StatusCreated)
}

// Get returns a single subject.
func (sh SubjectHandler) Get(c *echo.Context) error {
	request := new(GetSubjectRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	subject, err := sh.subjects.Get(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Key)
	if err != nil {
		return subjectProblem(c, err)
	}

	return c.JSON(http.StatusOK, subject)
}

// List returns all subjects for a tenant.
func (sh SubjectHandler) List(c *echo.Context) error {
	request := new(ListSubjectsRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	subjectList, err := sh.subjects.List(c.Request().Context(), utils.TenantOrDefault(request.TenantID))
	if err != nil {
		return subjectProblem(c, err)
	}

	return c.JSON(http.StatusOK, subjectList)
}

// Update replaces a subject's email and attributes.
func (sh SubjectHandler) Update(c *echo.Context) error {
	request := new(SubjectRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := sh.subjects.Update(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Key,
		request.Email, request.Attributes)
	if err != nil {
		return subjectProblem(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// Delete removes a subject and its role assignments.
func (sh SubjectHandler) Delete(c *echo.Context) error {
	request := new(GetSubjectRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := sh.subjects.Delete(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Key)
	if err != nil {
		return subjectProblem(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// Roles returns the role keys assigned to a subject — the payload BulwarkAuth
// embeds as the JWT roles claim at token issuance.
func (sh SubjectHandler) Roles(c *echo.Context) error {
	request := new(GetSubjectRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	roleKeys, err := sh.assignments.RolesForSubject(c.Request().Context(), utils.TenantOrDefault(request.TenantID),
		request.Key)
	if err != nil {
		return subjectProblem(c, err)
	}
	if roleKeys == nil {
		roleKeys = []string{}
	}

	return c.JSON(http.StatusOK, SubjectRolesResponse{Roles: roleKeys})
}

func subjectProblem(c *echo.Context, err error) error {
	var notFound subjects.SubjectNotFoundError
	if errors.As(err, &notFound) {
		return c.JSON(http.StatusNotFound, problem.NewProblem("Subject not found", http.StatusNotFound, err))
	}
	var duplicate subjects.SubjectDuplicateError
	if errors.As(err, &duplicate) {
		return c.JSON(http.StatusConflict, problem.NewProblem("Duplicate subject", http.StatusConflict, err))
	}
	var invalid subjects.InvalidSubjectError
	if errors.As(err, &invalid) {
		return c.JSON(http.StatusBadRequest, problem.NewProblem("Invalid subject", http.StatusBadRequest, err))
	}

	httpError := problem.NewServerError(err)
	return c.JSON(httpError.Status, httpError)
}
