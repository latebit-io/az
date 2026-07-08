package check

import "github.com/labstack/echo/v5"

func CheckRoutes(e *echo.Echo, handler CheckHandler, middleware ...echo.MiddlewareFunc) {
	e.POST("/api/check", handler.Check, middleware...)
	e.POST("/api/check/bulk", handler.CheckBulk, middleware...)
}
