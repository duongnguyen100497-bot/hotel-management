package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/trandinh0506/hotel-management/backend/internal/platform/config"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/logging"
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

	deps := []platformhttp.DependencyCheck{
		{Name: "postgres", Check: tcpProbeFromURL(cfg.DatabaseURL, "5432")},
		{Name: "redis", Check: redisPingProbe(cfg.RedisURL)},
	}
	if cfg.MinIOEnabled {
		deps = append(deps, platformhttp.DependencyCheck{
			Name:  "minio",
			Check: minioProbe(cfg.MinIOEndpoint, cfg.MinIOUseSSL),
		})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", platformhttp.HealthzHandler())
	mux.HandleFunc("GET /readyz", platformhttp.ReadyzHandler(3*time.Second, deps...))

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
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

func tcpProbeFromURL(rawURL, defaultPort string) platformhttp.CheckFunc {
	return func(ctx context.Context) error {
		u, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		host := u.Host
		if host == "" {
			return errors.New("empty host")
		}
		if _, _, err := net.SplitHostPort(host); err != nil {
			host = net.JoinHostPort(host, defaultPort)
		}
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", host)
		if err != nil {
			return err
		}
		return conn.Close()
	}
}

func redisPingProbe(rawURL string) platformhttp.CheckFunc {
	return func(ctx context.Context) error {
		u, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		host := u.Host
		if host == "" {
			return errors.New("empty redis host")
		}
		if _, _, err := net.SplitHostPort(host); err != nil {
			host = net.JoinHostPort(host, "6379")
		}
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", host)
		if err != nil {
			return err
		}
		defer conn.Close()

		if dl, ok := ctx.Deadline(); ok {
			_ = conn.SetDeadline(dl)
		}
		if _, err := conn.Write([]byte("*1\r\n$4\r\nPING\r\n")); err != nil {
			return err
		}
		line, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			return err
		}
		if !strings.HasPrefix(line, "+PONG") {
			return fmt.Errorf("unexpected redis reply: %q", strings.TrimSpace(line))
		}
		return nil
	}
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
