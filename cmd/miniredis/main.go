package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/janmang8225/mini-redis/config"
	"github.com/janmang8225/mini-redis/internal/server"
	"github.com/janmang8225/mini-redis/internal/store"
)

var version = "0.1.0"

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("miniredis", version)
		os.Exit(0)
	}

	// load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	// setup structured logger
	level := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})))

	slog.Info("starting miniredis", "version", version, "port", cfg.Port)

	// init store
	st := store.New()

	// start active expiry worker — cleans expired keys every 100ms
	expiryDone := make(chan struct{})
	st.StartExpiryWorker(100*time.Millisecond, expiryDone)

	// init TCP server
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := server.New(addr, st)

	// graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		slog.Info("shutting down...")
		close(expiryDone)
		srv.Stop()
	}()

	// blocking — runs until Stop() is called
	if err := srv.Start(); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}