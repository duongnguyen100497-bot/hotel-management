package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	auditpg "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/persistence/postgres"
	bookingapp "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/application"
	bookinghttp "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/delivery/http"
	bookingpg "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/persistence/postgres"
	bookingredis "github.com/trandinh0506/hotel-management/backend/internal/modules/booking/persistence/redis"
	identityapp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/application"
	identityhttp "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/delivery/http"
	identitypg "github.com/trandinh0506/hotel-management/backend/internal/modules/identity/persistence/postgres"
	mediaapp "github.com/trandinh0506/hotel-management/backend/internal/modules/media/application"
	mediahttp "github.com/trandinh0506/hotel-management/backend/internal/modules/media/delivery/http"
	mediapg "github.com/trandinh0506/hotel-management/backend/internal/modules/media/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/notification"
	peopleapp "github.com/trandinh0506/hotel-management/backend/internal/modules/people/application"
	peoplehttp "github.com/trandinh0506/hotel-management/backend/internal/modules/people/delivery/http"
	peoplepg "github.com/trandinh0506/hotel-management/backend/internal/modules/people/persistence/postgres"
	pricingapp "github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/application"
	pricinghttp "github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/delivery/http"
	pricingpg "github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/persistence/postgres"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	propertyhttp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/delivery/http"
	propertypg "github.com/trandinh0506/hotel-management/backend/internal/modules/property/persistence/postgres"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	rbachttp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/delivery/http"
	rbacpg "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/config"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/logging"
	platformminio "github.com/trandinh0506/hotel-management/backend/internal/platform/minio"
	platformredis "github.com/trandinh0506/hotel-management/backend/internal/platform/redis"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/smtp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	logger := logging.New(cfg.AppEnv, os.Stdout)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("server exited with error", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("server stopped cleanly")
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	var (
		workerWG sync.WaitGroup
		closers  []func() error
	)
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()

	pool, err := db.NewPool(ctx, db.PoolConfig{
		DatabaseURL: cfg.DatabaseURL,
		MaxConns:    cfg.DBMaxConns,
		MinConns:    cfg.DBMinConns,
	})
	if err != nil {
		return fmt.Errorf("init postgres pool: %w", err)
	}
	closers = append(closers, func() error {
		pool.Close()
		return nil
	})
	txRunner := db.NewTxRunner(pool)
	sysClock := clock.NewSystem()
	idGen := id.NewUUIDGenerator()

	auditSvc := auditapp.NewService(auditpg.NewRepository(pool), sysClock, idGen)
	outboxWorker := notification.NewWorker(txRunner, auditSvc, sysClock, logger, notification.WorkerConfig{
		PollInterval: time.Second,
		BatchSize:    20,
	})

	payloadCipher, err := auth.NewPayloadCipher(cfg.JWTRefreshSecret)
	if err != nil {
		return fmt.Errorf("init outbox payload cipher: %w", err)
	}
	smtpClient := smtp.NewClient(smtp.Config{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUser,
		Password: cfg.SMTPPass,
		From:     cfg.SMTPFrom,
	})
	_ = notification.NewEmailDispatcher(notification.EmailDispatcherConfig{
		Sender:          smtpClient,
		Cipher:          payloadCipher,
		FrontendBaseURL: cfg.FrontendBaseURL,
		PublicBaseURL:   cfg.PublicBaseURL,
	}, outboxWorker)

	workerWG.Add(1)
	go func() {
		defer workerWG.Done()
		outboxWorker.Run(workerCtx)
	}()

	redisClient, err := platformredis.NewClient(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("init redis client: %w", err)
	}
	closers = append(closers, redisClient.Close)

	deps := []platformhttp.DependencyCheck{
		{Name: "postgres", Check: db.PingCheck(pool)},
		{Name: "redis", Check: redisClient.PingCheck()},
	}
	if cfg.MinIOEnabled {
		deps = append(deps, platformhttp.DependencyCheck{
			Name:  "minio",
			Check: minioProbe(cfg.MinIOEndpoint, cfg.MinIOUseSSL),
		})
	}

	var minioStorage platformminio.Storage
	if cfg.MinIOEnabled {
		mc, err := platformminio.NewClient(platformminio.Config{
			Endpoint:      cfg.MinIOEndpoint,
			AccessKey:     cfg.MinIOAccessKey,
			SecretKey:     cfg.MinIOSecretKey,
			Bucket:        cfg.MinIOBucket,
			UseSSL:        cfg.MinIOUseSSL,
			PublicBaseURL: cfg.MinIOPublicBaseURL,
		})
		if err != nil {
			return fmt.Errorf("init minio client: %w", err)
		}
		minioStorage = mc
	}

	tokenManager := auth.NewTokenManager(cfg.JWTAccessSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL, sysClock)
	identitySvc := identityapp.NewService(txRunner, identitypg.NewRepository(pool), auditSvc, auditSvc, sysClock, idGen).
		WithTokenManager(tokenManager).
		WithPayloadCipher(payloadCipher)
	rbacSvc := rbacapp.NewService(txRunner, rbacpg.NewRepository(pool), identitySvc, auditSvc, sysClock, idGen)
	peopleSvc := peopleapp.NewService(txRunner, peoplepg.NewRepository(pool), identitySvc, rbacSvc, auditSvc, auditSvc, sysClock, idGen)
	identitySvc.SetCustomerRegistrationHook(peopleSvc)
	propertySvc := propertyapp.NewService(txRunner, propertypg.NewRepository(pool), rbacSvc, auditSvc, sysClock, idGen)
	pricingSvc := pricingapp.NewService(txRunner, pricingpg.NewRepository(pool), propertySvc, rbacSvc, auditSvc, sysClock, idGen)
	mediaSvc := mediaapp.NewService(txRunner, mediapg.NewRepository(pool), minioStorage, propertySvc, rbacSvc, auditSvc, auditSvc, sysClock, idGen).
		WithUploadMaxBytes(cfg.UploadMaxBytes).
		WithLogger(logger)
	propertySvc.SetMediaResolver(mediaSvc)
	bookingSvc := bookingapp.NewService(txRunner, bookingpg.NewRepository(pool), propertySvc, propertySvc, pricingSvc, peopleSvc, rbacSvc, auditSvc, auditSvc, sysClock, idGen).
		WithHoldStore(bookingredis.NewHoldStore(redisClient)).
		WithLogger(logger)
	propertySvc.SetAvailabilityReader(bookingSvc)
	peopleSvc.SetCustomerArchiveChecker(bookingSvc)
	outboxWorker.Register(mediaapp.EventMediaObjectDeleteRequested, mediaSvc.HandleObjectDeleteRequested)

	workerWG.Add(1)
	go func() {
		defer workerWG.Done()
		mediaSvc.RunCleanupWorker(workerCtx, 15*time.Minute)
	}()

	workerWG.Add(1)
	go func() {
		defer workerWG.Done()
		bookingSvc.RunExpiryScanner(workerCtx, 10*time.Second)
	}()

	workerWG.Add(1)
	go func() {
		defer workerWG.Done()
		bookingSvc.RunReconciler(workerCtx, 90*time.Second)
	}()

	authMiddleware := auth.AuthenticatorMiddleware(tokenManager, identitySvc)
	identityHandler := identityhttp.NewHandler(identityhttp.HandlerConfig{
		Service:      identitySvc,
		Tokens:       tokenManager,
		SecureCookie: cfg.AppEnv == config.EnvProduction,
	})
	rbacHandler := rbachttp.NewHandler(rbacSvc)
	peopleHandler := peoplehttp.NewHandler(peopleSvc, rbacSvc)
	propertyHandler := propertyhttp.NewHandler(propertyhttp.HandlerConfig{
		Service:    propertySvc,
		Authorizer: rbacSvc,
		Tokens:     tokenManager,
		Verifier:   identitySvc,
	})
	pricingHandler := pricinghttp.NewHandler(pricingSvc, rbacSvc)
	mediaHandler := mediahttp.NewHandler(mediaSvc, rbacSvc)
	bookingHandler := bookinghttp.NewHandler(bookingSvc, rbacSvc)

	router := platformhttp.NewRouter(platformhttp.RouterConfig{
		Logger:             logger,
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
		BodyLimitBytes:     platformhttp.DefaultBodyLimitBytes,
		ReadyzTimeout:      3 * time.Second,
		ReadyzChecks:       deps,
	}, func(api *gin.RouterGroup) {
		identityHandler.RegisterRoutes(api, authMiddleware)
		rbacHandler.RegisterRoutes(api, authMiddleware)
		peopleHandler.RegisterRoutes(api, authMiddleware)
		propertyHandler.RegisterRoutes(api, authMiddleware)
		pricingHandler.RegisterRoutes(api, authMiddleware)
		mediaHandler.RegisterRoutes(api, authMiddleware)
		bookingHandler.RegisterRoutes(api, authMiddleware)
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", slog.String("addr", cfg.HTTPAddr), slog.String("env", cfg.AppEnv))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	var serveErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		serveErr = err
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown error", slog.Any("error", err))
		serveErr = errors.Join(serveErr, err)
	}

	cancelWorkers()
	_ = workerCtx
	workerWG.Wait()

	for i := len(closers) - 1; i >= 0; i-- {
		if err := closers[i](); err != nil {
			logger.Error("cleanup closer failed", slog.Any("error", err))
			serveErr = errors.Join(serveErr, err)
		}
	}

	return serveErr
}

func minioProbe(endpoint string, useSSL bool) platformhttp.CheckFunc {
	client := &http.Client{Timeout: 3 * time.Second}
	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	target := fmt.Sprintf("%s://%s/minio/health/live", scheme, strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://"))
	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("minio health status %d", resp.StatusCode)
		}
		return nil
	}
}
