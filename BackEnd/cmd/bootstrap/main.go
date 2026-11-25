package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	auditpg "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/persistence/postgres"
	identityapp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/application"
	identitypg "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/persistence/postgres"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	rbacpg "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/config"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/logging"
)

func main() {
	appEnv := strings.TrimSpace(os.Getenv("APP_ENV"))
	if appEnv == "" {
		appEnv = config.EnvDevelopment
	}
	logger := logging.New(appEnv, os.Stdout)
	slog.SetDefault(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := runBootstrap(ctx, os.LookupEnv, logger); err != nil {
		logger.Error("bootstrap failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func runBootstrap(ctx context.Context, lookup func(string) (string, bool), logger *slog.Logger) error {
	requireEnv := func(key string) (string, error) {
		v, ok := lookup(key)
		if !ok || strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("missing required env %s", key)
		}
		return strings.TrimSpace(v), nil
	}

	dbURL, err := requireEnv("DATABASE_URL")
	if err != nil {
		return err
	}
	adminEmail, err := requireEnv("BOOTSTRAP_ADMIN_EMAIL")
	if err != nil {
		return err
	}
	adminPass, err := requireEnv("BOOTSTRAP_ADMIN_PASSWORD")
	if err != nil {
		return err
	}
	adminName, ok := lookup("BOOTSTRAP_ADMIN_NAME")
	if !ok || strings.TrimSpace(adminName) == "" {
		adminName = "System Administrator"
	}

	pool, err := db.NewPool(ctx, db.PoolConfig{
		DatabaseURL: dbURL,
		MaxConns:    5,
		MinConns:    1,
	})
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	txRunner := db.NewTxRunner(pool)
	sysClock := clock.NewSystem()
	idGen := id.NewUUIDGenerator()

	auditSvc := auditapp.NewService(auditpg.NewRepository(pool), sysClock, idGen)
	identitySvc := identityapp.NewService(txRunner, identitypg.NewRepository(pool), auditSvc, auditSvc, sysClock, idGen)
	rbacSvc := rbacapp.NewService(txRunner, rbacpg.NewRepository(pool), identitySvc, auditSvc, sysClock, idGen)

	res, err := rbacSvc.BootstrapRootAdmin(ctx, rbacapp.BootstrapAdminParams{
		Email:       adminEmail,
		Password:    adminPass,
		DisplayName: strings.TrimSpace(adminName),
	})
	if err != nil {
		return err
	}

	if res.CreatedRootAdmin {
		logger.Info("rbac seed and initial root administrator created",
			slog.String("root_user_id", res.RootUserID.String()),
			slog.String("root_role_id", res.RootRoleID.String()),
		)
	} else {
		logger.Info("rbac seed verified; active root administrator already exists (skipped creation)",
			slog.String("root_user_id", res.RootUserID.String()),
			slog.String("root_role_id", res.RootRoleID.String()),
		)
	}
	return nil
}
