package check

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/latebit-io/az/api/problem"
	"github.com/latebit-io/az/internal/check"
	"github.com/latebit-io/az/internal/utils"
)

type CheckHandler struct {
	check check.CheckService
}

type CheckRequest struct {
	TenantID string              `json:"tenantId"`
	Subject  check.CheckSubject  `json:"subject"`
	Action   string              `json:"action"`
	Resource check.CheckResource `json:"resource"`
}

type BulkCheckRequest struct {
	TenantID string `json:"tenantId"`
	Checks   []struct {
		Subject  check.CheckSubject  `json:"subject"`
		Action   string              `json:"action"`
		Resource check.CheckResource `json:"resource"`
	} `json:"checks"`
}

type BulkCheckResponse struct {
	Results []check.Decision `json:"results"`
}

func NewCheckHandler(service check.CheckService) CheckHandler {
	return CheckHandler{service}
}

// Check decides whether a subject may perform an action on a resource.
// Denies return 200 with allow:false — callers branch on the decision.
func (ch CheckHandler) Check(c *echo.Context) error {
	request := new(CheckRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	decision, err := ch.check.Check(c.Request().Context(), utils.TenantOrDefault(request.TenantID),
		check.CheckRequest{Subject: request.Subject, Action: request.Action, Resource: request.Resource})
	if err != nil {
		return checkProblem(c, err)
	}

	return c.JSON(http.StatusOK, decision)
}

// CheckBulk decides a batch of checks; results come back in request order.
func (ch CheckHandler) CheckBulk(c *echo.Context) error {
	request := new(BulkCheckRequest)
	if err := c.Bind(request); err != nil {
		httpError := problem.NewBadRequest(err)
		return c.JSON(httpError.Status, httpError)
	}

	requests := make([]check.CheckRequest, 0, len(request.Checks))
	for _, item := range request.Checks {
		requests = append(requests, check.CheckRequest{
			Subject:  item.Subject,
			Action:   item.Action,
			Resource: item.Resource,
		})
	}

	decisions, err := ch.check.CheckBulk(c.Request().Context(), utils.TenantOrDefault(request.TenantID), requests)
	if err != nil {
		return checkProblem(c, err)
	}

	return c.JSON(http.StatusOK, BulkCheckResponse{Results: decisions})
}

func checkProblem(c *echo.Context, err error) error {
	var invalid check.InvalidCheckError
	if errors.As(err, &invalid) {
		return c.JSON(http.StatusBadRequest, problem.NewProblem("Invalid check", http.StatusBadRequest, err))
	}

	httpError := problem.NewServerError(err)
	return c.JSON(httpError.Status, httpError)
}
