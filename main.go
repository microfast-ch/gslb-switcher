package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/microfast-ch/gslb-switcher/internal/checkers"
	"github.com/microfast-ch/gslb-switcher/internal/gslb"
	"github.com/microfast-ch/gslb-switcher/internal/opnsense"
)

func main() {
	// Global Configuration
	gslbHost := os.Getenv("GSLB_HOST")
	gslbPrimary := os.Getenv("GSLB_PRIMARY_IP")
	gslbPrimaryCheck := os.Getenv("GSLB_PRIMARY_CHECK")
	gslbSecondary := os.Getenv("GSLB_SECONDARY_IP")

	if gslbHost == "" || gslbPrimary == "" || gslbPrimaryCheck == "" || gslbSecondary == "" {
		slog.Error("missing required environment variables",
			slog.String("GSLB_HOST", gslbHost),
			slog.String("GSLB_PRIMARY_IP", gslbPrimary),
			slog.String("GSLB_PRIMARY_CHECK", gslbPrimaryCheck),
			slog.String("GSLB_SECONDARY_IP", gslbSecondary),
		)
		os.Exit(1)
	}

	// Create checker, currently only SimpleHTTPChecker is supported
	chk := checkers.NewSimpleHTTPChecker(gslbPrimaryCheck)

	cfg := gslb.GslbConfig{
		Host:                 gslbHost,
		PrimaryIP:            gslbPrimary,
		SecondaryIP:          gslbSecondary,
		PrimaryHealthChecker: chk,
	}

	// We currently only support OpnSense as GSLB provider
	opnsenseHost := os.Getenv("OPNSENSE_HOST")
	opnsenseAuth := os.Getenv("OPNSENSE_AUTH")

	if opnsenseHost == "" || opnsenseAuth == "" {
		slog.Error("missing required OpnSense environment variables",
			slog.String("OPNSENSE_HOST", opnsenseHost),
			slog.String("OPNSENSE_AUTH", "masked"),
		)
		os.Exit(1)
	}

	p, err := opnsense.NewOpnSenseGslb(opnsenseHost, opnsenseAuth, cfg)
	if err != nil {
		slog.Error("error creating GSLB provider", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Start GSLB
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		slog.Info("shutting down gracefully")
		cancel()
	}()

	if err := gslb.Run(ctx, p, 60*time.Second); err != nil && err != context.Canceled {
		slog.Error("error running GSLB", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
