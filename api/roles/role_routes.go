package roles

import "github.com/labstack/echo/v5"

func RoleRoutes(e *echo.Echo, handler RoleHandler, middleware ...echo.MiddlewareFunc) {
	e.POST("/api/roles", handler.Create, middleware...)
	e.POST("/api/roles/get", handler.Get, middleware...)
	e.POST("/api/roles/list", handler.List, middleware...)
	e.PUT("/api/roles", handler.Update, middleware...)
	e.PUT("/api/roles/delete", handler.Delete, middleware...)
}
