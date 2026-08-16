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
	"runtime/debug"
	"syscall"
	"time"

	"github.com/techfish-lab/kata-exporter/internal/app"
	"github.com/techfish-lab/kata-exporter/internal/config"
	"github.com/techfish-lab/kata-exporter/internal/install"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "kata-exporter:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "serve" {
		if len(args) > 0 {
			args = args[1:]
		}
		return serve(args)
	}
	switch args[0] {
	case "check":
		return check(args[1:])
	case "install":
		return install.Run(args[1:], version)
	case "uninstall":
		return install.Uninstall(args[1:])
	case "print-dashboard":
		fmt.Print(install.DashboardJSON())
		return nil
	case "version", "--version", "-v":
		fmt.Println(versionString())
		return nil
	case "help", "--help", "-h":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q (try: kata-exporter help)", args[0])
	}
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "JSON config path")
	listen := fs.String("listen", "", "override listen address")
	logLevel := fs.String("log-level", "info", "debug, info, warn, or error")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if *listen != "" {
		cfg.Listen = *listen
	}
	logger := newLogger(*logLevel)
	service, err := app.New(cfg, logger, version)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           service.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	logger.Info("Kata Exporter started", "listen", cfg.Listen, "metrics", "/metrics")
	err = srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func check(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "JSON config path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return app.Check(ctx, cfg, os.Stdout)
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	switch level {
	case "debug": l = slog.LevelDebug
	case "warn": l = slog.LevelWarn
	case "error": l = slog.LevelError
	default: l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: l}))
}

func versionString() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func usage() {
	fmt.Print(`Kata Exporter - Prometheus exporter for SwitchBot Kata Friends

Usage:
  kata-exporter serve [--config PATH] [--listen ADDR]
  kata-exporter check [--config PATH]
  sudo -E kata-exporter install [options]
  sudo kata-exporter uninstall [--purge]
  kata-exporter print-dashboard
  kata-exporter version

Configuration can come from a JSON file or KATA_* environment variables.
Run "kata-exporter install --help" for the all-in-one installer.
`)
}

