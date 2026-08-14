package main

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/rossigee/monero-exporter/internal/collector"
	"github.com/rossigee/monero-exporter/internal/rpc"
)

// runExporter wires the RPC client, registers a single prometheus.Collector
// implementation, and serves /metrics until a SIGINT/SIGTERM arrives.
func runExporter(cfg config, log *logrus.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cli, err := rpc.New(cfg.MoneroAddr, cfg.RPCUser, cfg.RPCPassword, log)
	if err != nil {
		return fmt.Errorf("rpc client: %w", err)
	}

	pingCtx, pingCancel := context.WithTimeout(ctx, 15*time.Second)
	if err := cli.Ping(pingCtx); err != nil {
		pingCancel()
		return fmt.Errorf("rpc ping: %w", err)
	}
	pingCancel()

	col, err := collector.Register(cli, log)
	if err != nil {
		return fmt.Errorf("collector register: %w", err)
	}

	mux := newMux(col, cfg.TelemetryPath, prometheus.DefaultGatherer)

	srv := &http.Server{
		Addr:              cfg.BindAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		log.Info("shutdown signal received; draining")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.WithFields(logrus.Fields{
		"bind_addr":      cfg.BindAddr,
		"telemetry_path": cfg.TelemetryPath,
		"monero_addr":    cfg.MoneroAddr,
	}).Info("monero-exporter starting")

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

// newMux builds the HTTP routes. The /metrics handler refreshes the cached
// monerod snapshot before serving it, and /healthz answers liveness probes.
// gatherer supplies the scrape endpoint (normally prometheus.DefaultGatherer).
func newMux(col *collector.Collector, telemetryPath string, gatherer prometheus.Gatherer) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle(telemetryPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		col.Refresh(refreshCtx)
		promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}
