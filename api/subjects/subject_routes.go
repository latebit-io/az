package subjects

import "github.com/labstack/echo/v5"

func SubjectRoutes(e *echo.Echo, handler SubjectHandler, middleware ...echo.MiddlewareFunc) {
	e.POST("/api/subjects", handler.Create, middleware...)
	e.POST("/api/subjects/get", handler.Get, middleware...)
	e.POST("/api/subjects/list", handler.List, middleware...)
	e.POST("/api/subjects/roles", handler.Roles, middleware...)
	e.PUT("/api/subjects", handler.Update, middleware...)
	e.PUT("/api/subjects/delete", handler.Delete, middleware...)
}
