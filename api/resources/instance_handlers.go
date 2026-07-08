package resources

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/latebit-io/az/api/problem"
	"github.com/latebit-io/az/internal/resources"
	"github.com/latebit-io/az/internal/utils"
)

type InstanceHandler struct {
	instances resources.InstanceService
}

type InstanceRequest struct {
	TenantID     string         `json:"tenantId"`
	ResourceType string         `json:"resourceType"`
	Key          string         `json:"key"`
	Attributes   map[string]any `json:"attributes"`
}

type GetInstanceRequest struct {
	TenantID     string `json:"tenantId"`
	ResourceType string `json:"resourceType"`
	Key          string `json:"key"`
}

type ListInstancesRequest struct {
	TenantID     string `json:"tenantId"`
	ResourceType string `json:"resourceType"`
}

func NewInstanceHandler(service resources.InstanceService) InstanceHandler {
	return InstanceHandler{service}
}

// Create registers a resource instance with attributes for ABAC checks.
func (ih InstanceHandler) Create(c *echo.Context) error {
	request := new(InstanceRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := ih.instances.Create(c.Request().Context(), utils.TenantOrDefault(request.TenantID),
		request.ResourceType, request.Key, request.Attributes)
	if err != nil {
		return instanceProblem(c, err)
	}

	return c.NoContent(http.StatusCreated)
}

// Get returns a single resource instance.
func (ih InstanceHandler) Get(c *echo.Context) error {
	request := new(GetInstanceRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	instance, err := ih.instances.Get(c.Request().Context(), utils.TenantOrDefault(request.TenantID),
		request.ResourceType, request.Key)
	if err != nil {
		return instanceProblem(c, err)
	}

	return c.JSON(http.StatusOK, instance)
}

// List returns all instances of a resource type.
func (ih InstanceHandler) List(c *echo.Context) error {
	request := new(ListInstancesRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	instances, err := ih.instances.List(c.Request().Context(), utils.TenantOrDefault(request.TenantID),
		request.ResourceType)
	if err != nil {
		return instanceProblem(c, err)
	}

	return c.JSON(http.StatusOK, instances)
}

// Update replaces a resource instance's attributes.
func (ih InstanceHandler) Update(c *echo.Context) error {
	request := new(InstanceRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := ih.instances.Update(c.Request().Context(), utils.TenantOrDefault(request.TenantID),
		request.ResourceType, request.Key, request.Attributes)
	if err != nil {
		return instanceProblem(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// Delete removes a resource instance.
func (ih InstanceHandler) Delete(c *echo.Context) error {
	request := new(GetInstanceRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := ih.instances.Delete(c.Request().Context(), utils.TenantOrDefault(request.TenantID),
		request.ResourceType, request.Key)
	if err != nil {
		return instanceProblem(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func instanceProblem(c *echo.Context, err error) error {
	var instanceNotFound resources.InstanceNotFoundError
	if errors.As(err, &instanceNotFound) {
		return c.JSON(http.StatusNotFound, problem.NewProblem("Resource instance not found", http.StatusNotFound, err))
	}
	var typeNotFound resources.ResourceTypeNotFoundError
	if errors.As(err, &typeNotFound) {
		return c.JSON(http.StatusNotFound, problem.NewProblem("Resource type not found", http.StatusNotFound, err))
	}
	var duplicate resources.InstanceDuplicateError
	if errors.As(err, &duplicate) {
		return c.JSON(http.StatusConflict, problem.NewProblem("Duplicate resource instance", http.StatusConflict, err))
	}
	var invalid resources.InvalidInstanceError
	if errors.As(err, &invalid) {
		return c.JSON(http.StatusBadRequest, problem.NewProblem("Invalid resource instance", http.StatusBadRequest, err))
	}

	httpError := problem.NewServerError(err)
	return c.JSON(httpError.Status, httpError)
}
