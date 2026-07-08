package conditions

import "github.com/labstack/echo/v5"

func ConditionSetRoutes(e *echo.Echo, handler ConditionSetHandler, middleware ...echo.MiddlewareFunc) {
	e.POST("/api/conditionsets", handler.Create, middleware...)
	e.POST("/api/conditionsets/get", handler.Get, middleware...)
	e.POST("/api/conditionsets/list", handler.List, middleware...)
	e.PUT("/api/conditionsets", handler.Update, middleware...)
	e.PUT("/api/conditionsets/delete", handler.Delete, middleware...)
}
