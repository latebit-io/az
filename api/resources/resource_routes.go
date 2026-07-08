package resources

import "github.com/labstack/echo/v5"

func ResourceRoutes(e *echo.Echo, handler ResourceHandler, middleware ...echo.MiddlewareFunc) {
	e.POST("/api/resources", handler.Create, middleware...)
	e.POST("/api/resources/get", handler.Get, middleware...)
	e.POST("/api/resources/list", handler.List, middleware...)
	e.PUT("/api/resources", handler.Update, middleware...)
	e.PUT("/api/resources/delete", handler.Delete, middleware...)
}
