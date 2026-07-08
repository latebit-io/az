package roles

import "github.com/labstack/echo/v5"

func AssignmentRoutes(e *echo.Echo, handler AssignmentHandler, middleware ...echo.MiddlewareFunc) {
	e.POST("/api/assignments", handler.Assign, middleware...)
	e.POST("/api/assignments/list", handler.List, middleware...)
	e.PUT("/api/assignments/delete", handler.Unassign, middleware...)
}
