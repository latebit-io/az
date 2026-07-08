package conditions

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/latebit-io/az/api/problem"
	"github.com/latebit-io/az/internal/conditions"
	"github.com/latebit-io/az/internal/roles"
	"github.com/latebit-io/az/internal/utils"
)

type RuleHandler struct {
	rules conditions.RuleService
}

type RuleRequest struct {
	TenantID    string           `json:"tenantId"`
	SubjectSet  string           `json:"subjectSet"`
	Permission  roles.Permission `json:"permission"`
	ResourceSet string           `json:"resourceSet"`
}

type ListRulesRequest struct {
	TenantID string `json:"tenantId"`
}

func NewRuleHandler(service conditions.RuleService) RuleHandler {
	return RuleHandler{service}
}

// Create grants a permission to a subject set on a resource set.
func (h RuleHandler) Create(c *echo.Context) error {
	request := new(RuleRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := h.rules.Create(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.SubjectSet,
		request.Permission.Resource, request.Permission.Action, request.ResourceSet)
	if err != nil {
		return ruleProblem(c, err)
	}

	return c.NoContent(http.StatusCreated)
}

// List returns all condition set rules for a tenant.
func (h RuleHandler) List(c *echo.Context) error {
	request := new(ListRulesRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	rules, err := h.rules.List(c.Request().Context(), utils.TenantOrDefault(request.TenantID))
	if err != nil {
		return ruleProblem(c, err)
	}

	return c.JSON(http.StatusOK, rules)
}

// Delete removes a condition set rule.
func (h RuleHandler) Delete(c *echo.Context) error {
	request := new(RuleRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	err := h.rules.Delete(c.Request().Context(), utils.TenantOrDefault(request.TenantID), request.SubjectSet,
		request.Permission.Resource, request.Permission.Action, request.ResourceSet)
	if err != nil {
		return ruleProblem(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func ruleProblem(c *echo.Context, err error) error {
	var notFound conditions.RuleNotFoundError
	if errors.As(err, &notFound) {
		return c.JSON(http.StatusNotFound, problem.NewProblem("Rule not found", http.StatusNotFound, err))
	}
	var duplicate conditions.RuleDuplicateError
	if errors.As(err, &duplicate) {
		return c.JSON(http.StatusConflict, problem.NewProblem("Duplicate rule", http.StatusConflict, err))
	}
	var invalid conditions.InvalidRuleError
	if errors.As(err, &invalid) {
		return c.JSON(http.StatusBadRequest, problem.NewProblem("Invalid rule", http.StatusBadRequest, err))
	}

	httpError := problem.NewServerError(err)
	return c.JSON(httpError.Status, httpError)
}
