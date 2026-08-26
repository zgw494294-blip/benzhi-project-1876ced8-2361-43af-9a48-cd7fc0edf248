package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"rigging-readiness-desk/internal/application"
	"rigging-readiness-desk/internal/storage"
	webui "rigging-readiness-desk/internal/web"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(os.Args[1:], logger); err != nil {
		logger.Error("service_exit", "error", err)
		os.Exit(1)
	}
}
func run(args []string, logger *slog.Logger) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	dataPath := cfg.DataPath
	if cfg.Mode == "selfcheck" {
		dataPath = ":memory:"
	}
	store, err := storage.Open(dataPath)
	if err != nil {
		return err
	}
	service := application.NewService(store)
	defer service.Close()
	handler := webui.NewHandler(service, logger)
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 45 * time.Second}
	serveErr := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()
	logger.Info("service_started", "addr", listener.Addr().String(), "mode", cfg.Mode)
	if cfg.Mode == "selfcheck" {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		defer cancel()
		checkErr := runSelfcheck(ctx, "http://"+listener.Addr().String())
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		serveResult := <-serveErr
		if checkErr != nil {
			return checkErr
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		if serveResult != nil {
			return serveResult
		}
		logger.Info("selfcheck_complete", "status", "ok")
		return nil
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case sig := <-signals:
		logger.Info("shutdown_requested", "signal", sig.String())
	case err := <-serveErr:
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	return server.Shutdown(ctx)
}
