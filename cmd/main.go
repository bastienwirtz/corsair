package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/bastienwirtz/corsair/config"
	"github.com/bastienwirtz/corsair/server"
)

var version = "dev"

func main() {
	configPath := flag.String("c", config.DEFAULT_PATH, "path to configuration file")
	versionFlag := flag.Bool("version", false, "print version information")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Version: %s\n", version)
		return
	}
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		slog.Error("Failed to load config", "config_file", *configPath, "error", err)
		os.Exit(1)
	}

	if err := server.SetupLogger(cfg.Logging, version); err != nil {
		slog.Error("Failed to setup logger", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting Corsair", "config_file", *configPath, "log_level", cfg.Logging.Level)

	httpServer := &http.Server{
		Addr:         net.JoinHostPort(cfg.Server.Address, strconv.Itoa(cfg.Server.Port)),
		Handler:      server.NewDynamicRoutingHandler(*cfg),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine to ensure graceful shutdown
	go func() {
		slog.Info("Starting HTTP server",
			"address", cfg.Server.Address,
			"port", cfg.Server.Port,
			"read_timeout", httpServer.ReadTimeout,
			"write_timeout", httpServer.WriteTimeout)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err, "addr", httpServer.Addr)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan

	slog.Info("Received shutdown signal", "signal", sig.String())
	slog.Info("Server shutdown complete")
}
