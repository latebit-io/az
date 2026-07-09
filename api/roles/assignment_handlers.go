package roles

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/latebit-io/az/api/problem"
	"github.com/latebit-io/az/internal/roles"
	"github.com/latebit-io/az/internal/utils"
)

type AssignmentHandler struct {
	assignments roles.AssignmentService
}

type AssignmentRequest struct {
	TenantID string `json:"tenantId"`
	Subject  string `json:"subject"`
	RoleID   string `json:"roleId"`
}

type ListAssignmentsRequest struct {
	TenantID string `json:"tenantId"`
	Subject  string `json:"subject"`
	RoleID   string `json:"roleId"`
}

type SubjectRolesRequest struct {
	TenantID string `json:"tenantId"`
	Key      string `json:"key"`
}

type SubjectRolesResponse struct {
	Roles []string `json:"roles"`
}

func NewAssignmentHandler(service roles.AssignmentService) AssignmentHandler {
	return AssignmentHandler{service}
}

// Assign binds a role to a subject. The subject does not need to be stored;
// the role must exist.
func (ah AssignmentHandler) Assign(c *echo.Context) error {
	request := new(AssignmentRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := ah.assignments.Assign(c.Request().Context(), utils.TenantOrDefault(request.TenantID),
		request.Subject, request.RoleID)
	if err != nil {
		return assignmentProblem(c, err)
	}

	return c.NoContent(http.StatusCreated)
}

// List returns assignments, optionally filtered by subject and/or role.
func (ah AssignmentHandler) List(c *echo.Context) error {
	request := new(ListAssignmentsRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	assignments, err := ah.assignments.List(c.Request().Context(), utils.TenantOrDefault(request.TenantID),
		request.Subject, request.RoleID)
	if err != nil {
		return assignmentProblem(c, err)
	}

	return c.JSON(http.StatusOK, assignments)
}

// Unassign removes a role from a subject.
func (ah AssignmentHandler) Unassign(c *echo.Context) error {
	request := new(AssignmentRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := ah.assignments.Unassign(c.Request().Context(), utils.TenantOrDefault(request.TenantID),
		request.Subject, request.RoleID)
	if err != nil {
		return assignmentProblem(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// SubjectRoles returns the role keys assigned to a subject — the payload
// BulwarkAuth embeds as the JWT roles claim at token issuance.
func (ah AssignmentHandler) SubjectRoles(c *echo.Context) error {
	request := new(SubjectRolesRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	roleKeys, err := ah.assignments.RolesForSubject(c.Request().Context(), utils.TenantOrDefault(request.TenantID),
		request.Key)
	if err != nil {
		return assignmentProblem(c, err)
	}
	if roleKeys == nil {
		roleKeys = []string{}
	}

	return c.JSON(http.StatusOK, SubjectRolesResponse{Roles: roleKeys})
}

func assignmentProblem(c *echo.Context, err error) error {
	var assignmentNotFound roles.AssignmentNotFoundError
	if errors.As(err, &assignmentNotFound) {
		return c.JSON(http.StatusNotFound, problem.NewProblem("Assignment not found", http.StatusNotFound, err))
	}
	var roleNotFound roles.RoleNotFoundError
	if errors.As(err, &roleNotFound) {
		return c.JSON(http.StatusNotFound, problem.NewProblem("Role not found", http.StatusNotFound, err))
	}
	var duplicate roles.AssignmentDuplicateError
	if errors.As(err, &duplicate) {
		return c.JSON(http.StatusConflict, problem.NewProblem("Duplicate assignment", http.StatusConflict, err))
	}
	var invalid roles.InvalidAssignmentError
	if errors.As(err, &invalid) {
		return c.JSON(http.StatusBadRequest, problem.NewProblem("Invalid assignment", http.StatusBadRequest, err))
	}

	httpError := problem.NewServerError(err)
	return c.JSON(httpError.Status, httpError)
}
