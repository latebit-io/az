package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	checkapi "github.com/latebit-io/az/api/check"
	"github.com/latebit-io/az/api/health"
	resourcesapi "github.com/latebit-io/az/api/resources"
	rolesapi "github.com/latebit-io/az/api/roles"
	"github.com/latebit-io/az/internal/check"
	"github.com/latebit-io/az/internal/db"
	"github.com/latebit-io/az/internal/resources"
	"github.com/latebit-io/az/internal/roles"
	"github.com/latebit-io/az/internal/tenants"
	"github.com/latebit-io/az/internal/utils"
	"github.com/latebit-io/az/internal/version"
)

func main() {
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	flag.Parse()

	// If the version flag is passed, print the version and exit
	if *versionFlag {
		fmt.Println(version.GetVersionInfo())
		os.Exit(0)
	}

	logger := getLogger()
	fmt.Println(`
  __   ____
 / _\ (__  )
/    \ / _/
\_/\_/(____) v1.0.0`)
	err := godotenv.Load()
	if err != nil {
		logger.Warn("no .env file loading from system")
	}

	config, err := NewAppConfig()
	if err != nil {
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	service := echo.New()
	logger.Info("connecting to postgres", "uri", config.DbConnection)
	pool, err := db.Connect(ctx, config.DbConnection)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	logger.Info("running migrations")
	if err := db.Migrate(ctx, pool); err != nil {
		panic(err)
	}

	ratelimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(float64(config.RequestsPerSecond)))
	tenantRepository := tenants.NewPostgresTenantRepository(pool)
	tenantService := tenants.NewDefaultTenantService(tenantRepository)
	err = createDefaultTenantID(ctx, tenantService)
	if err != nil {
		panic(err)
	}
	txManager := utils.NewPostgresTxManager(pool)
	resourceRepository := resources.NewPostgresResourceTypeRepository(pool)
	resourceService := resources.NewDefaultResourceService(resourceRepository, txManager)
	resourceHandler := resourcesapi.NewResourceHandler(resourceService)
	resourcesapi.ResourceRoutes(service, resourceHandler, ratelimiter)
	roleRepository := roles.NewPostgresRoleRepository(pool)
	roleService := roles.NewDefaultRoleService(roleRepository, txManager)
	roleHandler := rolesapi.NewRoleHandler(roleService)
	rolesapi.RoleRoutes(service, roleHandler, ratelimiter)
	assignmentRepository := roles.NewPostgresAssignmentRepository(pool)
	assignmentService := roles.NewDefaultAssignmentService(assignmentRepository)
	assignmentHandler := rolesapi.NewAssignmentHandler(assignmentService)
	rolesapi.AssignmentRoutes(service, assignmentHandler, ratelimiter)
	checkService := check.NewDefaultCheckService(resourceRepository, roleRepository, assignmentRepository,
		logger, config.DecisionLogEnabled)
	checkHandler := checkapi.NewCheckHandler(checkService)
	checkapi.CheckRoutes(service, checkHandler, ratelimiter)

	corsSetting(service, config, logger)
	apiKeySetting(service, config, logger)

	healthHandler := health.NewHealthHandler()
	health.HealthRoutes(service, healthHandler)

	start := echo.StartConfig{
		Address:         fmt.Sprintf(":%d", config.Port),
		HideBanner:      true,
		GracefulTimeout: 10 * time.Second,
	}
	if err := start.Start(ctx, service); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error(err.Error())
	}
}

func getLogger() *slog.Logger {
	jsonHandler := slog.NewJSONHandler(os.Stderr, nil)
	logger := slog.New(jsonHandler)
	return logger
}

func createDefaultTenantID(ctx context.Context, tenantService tenants.TenantService) error {
	existingTenants, err := tenantService.ListTenants(ctx)
	if err != nil {
		return err
	}

	if len(existingTenants) == 0 {
		return tenantService.CreateDefault(ctx)
	}
	return nil
}

func corsSetting(service *echo.Echo, config *AppConfig, logger *slog.Logger) {
	if !config.CORSEnabled {
		return
	}
	if config.Domain != "" {
		config.AllowedOrigins = append(config.AllowedOrigins, fmt.Sprintf("https://%s", config.Domain))
	}

	service.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: config.AllowedOrigins,
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	}))

	logger.Info("cors enabled")
}

func apiKeySetting(service *echo.Echo, config *AppConfig, logger *slog.Logger) {
	if !config.ApiKeyEnabled {
		return
	}
	service.Use(middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		KeyLookup: "header:X-AZ-API-KEY",
		Validator: func(c *echo.Context, key string, source middleware.ExtractorSource) (bool, error) {
			return utils.SafeCompare(key, os.Getenv("API_KEY")), nil
		},
	}))
	logger.Info("api key enabled")
}
