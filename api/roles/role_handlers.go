package roles

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/latebit-io/az/api/problem"
	"github.com/latebit-io/az/internal/roles"
	"github.com/latebit-io/az/internal/utils"
)

type RoleHandler struct {
	roles roles.RoleService
}

type RoleRequest struct {
	TenantID    string             `json:"tenantId"`
	Key         string             `json:"key"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Permissions []roles.Permission `json:"permissions"`
}

type GetRoleRequest struct {
	TenantID string `json:"tenantId"`
	Key      string `json:"key"`
}

type ListRolesRequest struct {
	TenantID string `json:"tenantId"`
}

func NewRoleHandler(service roles.RoleService) RoleHandler {
	return RoleHandler{service}
}

// Create defines a role with its permission grants. Every grant is validated
// against the tenant's resource type definitions.
func (rh RoleHandler) Create(c *echo.Context) error {
	request := new(RoleRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := rh.roles.Create(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Key,
		request.Name, request.Description, request.Permissions)
	if err != nil {
		return roleProblem(c, err)
	}

	return c.NoContent(http.StatusCreated)
}

// Get returns a single role.
func (rh RoleHandler) Get(c *echo.Context) error {
	request := new(GetRoleRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	role, err := rh.roles.Get(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Key)
	if err != nil {
		return roleProblem(c, err)
	}

	return c.JSON(http.StatusOK, role)
}

// List returns all roles for a tenant.
func (rh RoleHandler) List(c *echo.Context) error {
	request := new(ListRolesRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	roleList, err := rh.roles.List(c.Request().Context(), utils.TenantOrDefault(request.TenantID))
	if err != nil {
		return roleProblem(c, err)
	}

	return c.JSON(http.StatusOK, roleList)
}

// Update replaces a role's name, description and permission grants.
func (rh RoleHandler) Update(c *echo.Context) error {
	request := new(RoleRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := rh.roles.Update(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Key,
		request.Name, request.Description, request.Permissions)
	if err != nil {
		return roleProblem(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// Delete removes a role and its assignments.
func (rh RoleHandler) Delete(c *echo.Context) error {
	request := new(GetRoleRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := rh.roles.Delete(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Key)
	if err != nil {
		return roleProblem(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func roleProblem(c *echo.Context, err error) error {
	var notFound roles.RoleNotFoundError
	if errors.As(err, &notFound) {
		return c.JSON(http.StatusNotFound, problem.NewProblem("Role not found", http.StatusNotFound, err))
	}
	var duplicate roles.RoleDuplicateError
	if errors.As(err, &duplicate) {
		return c.JSON(http.StatusConflict, problem.NewProblem("Duplicate role", http.StatusConflict, err))
	}
	var invalidRole roles.InvalidRoleError
	if errors.As(err, &invalidRole) {
		return c.JSON(http.StatusBadRequest, problem.NewProblem("Invalid role", http.StatusBadRequest, err))
	}
	var invalidPermission roles.InvalidPermissionError
	if errors.As(err, &invalidPermission) {
		return c.JSON(http.StatusBadRequest, problem.NewProblem("Invalid permission", http.StatusBadRequest, err))
	}

	httpError := problem.NewServerError(err)
	return c.JSON(httpError.Status, httpError)
}
