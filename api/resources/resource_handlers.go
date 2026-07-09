package resources

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/latebit-io/az/api/problem"
	"github.com/latebit-io/az/internal/resources"
	"github.com/latebit-io/az/internal/utils"
)

type ResourceHandler struct {
	resources resources.ResourceService
}

type ResourceTypeRequest struct {
	TenantID string   `json:"tenantId"`
	Key      string   `json:"key"`
	Actions  []string `json:"actions"`
}

type GetResourceTypeRequest struct {
	TenantID string `json:"tenantId"`
	Key      string `json:"key"`
}

type ListResourceTypesRequest struct {
	TenantID string `json:"tenantId"`
}

func NewResourceHandler(service resources.ResourceService) ResourceHandler {
	return ResourceHandler{service}
}

// Create defines a new resource type with its actions.
func (rh ResourceHandler) Create(c *echo.Context) error {
	request := new(ResourceTypeRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := rh.resources.Create(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Key,
		request.Actions)
	if err != nil {
		return resourceTypeProblem(c, err)
	}

	return c.NoContent(http.StatusCreated)
}

// Get returns a single resource type by tenant and key.
func (rh ResourceHandler) Get(c *echo.Context) error {
	request := new(GetResourceTypeRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	resourceType, err := rh.resources.Get(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Key)
	if err != nil {
		return resourceTypeProblem(c, err)
	}

	return c.JSON(http.StatusOK, resourceType)
}

// List returns all resource types for a tenant.
func (rh ResourceHandler) List(c *echo.Context) error {
	request := new(ListResourceTypesRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	resourceTypes, err := rh.resources.List(c.Request().Context(), utils.TenantOrDefault(request.TenantID))
	if err != nil {
		return resourceTypeProblem(c, err)
	}

	return c.JSON(http.StatusOK, resourceTypes)
}

// Update replaces a resource type's actions.
func (rh ResourceHandler) Update(c *echo.Context) error {
	request := new(ResourceTypeRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := rh.resources.Update(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Key,
		request.Actions)
	if err != nil {
		return resourceTypeProblem(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// Delete removes a resource type. Referenced resource types (roles, rules)
// cannot be deleted.
func (rh ResourceHandler) Delete(c *echo.Context) error {
	request := new(GetResourceTypeRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := rh.resources.Delete(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Key)
	if err != nil {
		return resourceTypeProblem(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func resourceTypeProblem(c *echo.Context, err error) error {
	var notFound resources.ResourceTypeNotFoundError
	if errors.As(err, &notFound) {
		return c.JSON(http.StatusNotFound, problem.NewProblem("Resource type not found", http.StatusNotFound, err))
	}
	var duplicate resources.ResourceTypeDuplicateError
	if errors.As(err, &duplicate) {
		return c.JSON(http.StatusConflict, problem.NewProblem("Duplicate resource type", http.StatusConflict, err))
	}
	var referenced resources.ResourceTypeReferencedError
	if errors.As(err, &referenced) {
		return c.JSON(http.StatusConflict, problem.NewProblem("Resource type referenced", http.StatusConflict, err))
	}
	var invalid resources.InvalidResourceTypeError
	if errors.As(err, &invalid) {
		return c.JSON(http.StatusBadRequest, problem.NewProblem("Invalid resource type", http.StatusBadRequest, err))
	}

	httpError := problem.NewServerError(err)
	return c.JSON(httpError.Status, httpError)
}
