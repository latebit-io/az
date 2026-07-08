package conditions

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/latebit-io/az/api/problem"
	"github.com/latebit-io/az/internal/conditions"
	"github.com/latebit-io/az/internal/utils"
)

type ConditionSetHandler struct {
	sets conditions.ConditionSetService
}

type ConditionSetRequest struct {
	TenantID     string                   `json:"tenantId"`
	Key          string                   `json:"key"`
	Name         string                   `json:"name"`
	Description  string                   `json:"description"`
	Type         string                   `json:"type"`
	ResourceType string                   `json:"resourceType"`
	Conditions   conditions.ConditionNode `json:"conditions"`
}

type GetConditionSetRequest struct {
	TenantID string `json:"tenantId"`
	Key      string `json:"key"`
}

type ListConditionSetsRequest struct {
	TenantID string `json:"tenantId"`
	Type     string `json:"type"`
}

func NewConditionSetHandler(service conditions.ConditionSetService) ConditionSetHandler {
	return ConditionSetHandler{service}
}

func (h ConditionSetHandler) setFromRequest(request *ConditionSetRequest) conditions.ConditionSet {
	return conditions.ConditionSet{
		TenantID:     utils.TenantOrDefault(request.TenantID),
		Key:          request.Key,
		Name:         request.Name,
		Description:  request.Description,
		Type:         request.Type,
		ResourceType: request.ResourceType,
		Conditions:   request.Conditions,
	}
}

// Create defines a subject or resource condition set.
func (h ConditionSetHandler) Create(c *echo.Context) error {
	request := new(ConditionSetRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	if err := h.sets.Create(c.Request().Context(), h.setFromRequest(request)); err != nil {
		return conditionSetProblem(c, err)
	}

	return c.NoContent(http.StatusCreated)
}

// Get returns a single condition set.
func (h ConditionSetHandler) Get(c *echo.Context) error {
	request := new(GetConditionSetRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	set, err := h.sets.Get(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Key)
	if err != nil {
		return conditionSetProblem(c, err)
	}

	return c.JSON(http.StatusOK, set)
}

// List returns condition sets, optionally filtered by type.
func (h ConditionSetHandler) List(c *echo.Context) error {
	request := new(ListConditionSetsRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	sets, err := h.sets.List(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Type)
	if err != nil {
		return conditionSetProblem(c, err)
	}

	return c.JSON(http.StatusOK, sets)
}

// Update replaces a condition set.
func (h ConditionSetHandler) Update(c *echo.Context) error {
	request := new(ConditionSetRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	if err := h.sets.Update(c.Request().Context(), h.setFromRequest(request)); err != nil {
		return conditionSetProblem(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// Delete removes a condition set. Sets referenced by rules cannot be deleted.
func (h ConditionSetHandler) Delete(c *echo.Context) error {
	request := new(GetConditionSetRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := h.sets.Delete(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.Key)
	if err != nil {
		return conditionSetProblem(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func conditionSetProblem(c *echo.Context, err error) error {
	var notFound conditions.ConditionSetNotFoundError
	if errors.As(err, &notFound) {
		return c.JSON(http.StatusNotFound, problem.NewProblem("Condition set not found", http.StatusNotFound, err))
	}
	var duplicate conditions.ConditionSetDuplicateError
	if errors.As(err, &duplicate) {
		return c.JSON(http.StatusConflict, problem.NewProblem("Duplicate condition set", http.StatusConflict, err))
	}
	var referenced conditions.ConditionSetReferencedError
	if errors.As(err, &referenced) {
		return c.JSON(http.StatusConflict, problem.NewProblem("Condition set referenced", http.StatusConflict, err))
	}
	var invalid conditions.InvalidConditionSetError
	if errors.As(err, &invalid) {
		return c.JSON(http.StatusBadRequest, problem.NewProblem("Invalid condition set", http.StatusBadRequest, err))
	}

	httpError := problem.NewServerError(err)
	return c.JSON(httpError.Status, httpError)
}
