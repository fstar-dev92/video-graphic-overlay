package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"video-graphic-overlay-gstreamer/internal/config"
	"video-graphic-overlay-gstreamer/internal/pipeline"
	"video-graphic-overlay-gstreamer/pkg/logger"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Initialize logger
	log := logger.New()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Infof("Starting video graphic overlay pipeline")
	log.Infof("HLS Input: %s", cfg.Input.HLSUrl)
	log.Infof("UDP Output: %s:%d", cfg.Output.Host, cfg.Output.Port)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create and start pipeline
	p, err := pipeline.New(cfg, log.Logger)
	if err != nil {
		log.Fatalf("Failed to create pipeline: %v", err)
	}

	// Start pipeline in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := p.Start(ctx); err != nil {
			errChan <- fmt.Errorf("pipeline error: %w", err)
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		log.Infof("Received signal %v, shutting down...", sig)
		cancel()
	case err := <-errChan:
		log.Errorf("Pipeline error: %v", err)
		cancel()
	}

	// Stop pipeline
	if err := p.Stop(); err != nil {
		log.Errorf("Error stopping pipeline: %v", err)
	}

	log.Info("Pipeline stopped successfully")
}
