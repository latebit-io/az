package conditions

import "github.com/labstack/echo/v5"

func RuleRoutes(e *echo.Echo, handler RuleHandler, middleware ...echo.MiddlewareFunc) {
	e.POST("/api/conditionsets/rules", handler.Create, middleware...)
	e.POST("/api/conditionsets/rules/list", handler.List, middleware...)
	e.PUT("/api/conditionsets/rules/delete", handler.Delete, middleware...)
}
