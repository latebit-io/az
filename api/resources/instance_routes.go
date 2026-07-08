package resources

import "github.com/labstack/echo/v5"

func InstanceRoutes(e *echo.Echo, handler InstanceHandler, middleware ...echo.MiddlewareFunc) {
	e.POST("/api/resources/instances", handler.Create, middleware...)
	e.POST("/api/resources/instances/get", handler.Get, middleware...)
	e.POST("/api/resources/instances/list", handler.List, middleware...)
	e.PUT("/api/resources/instances", handler.Update, middleware...)
	e.PUT("/api/resources/instances/delete", handler.Delete, middleware...)
}
