package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/rossigee/monero-exporter/internal/collector"
	"github.com/rossigee/monero-exporter/internal/rpc"
)

// runExporter wires the RPC client, registers a single prometheus.Collector
// implementation, and serves /metrics until ctx is cancelled. The caller owns
// ctx so tests can drive shutdown without signalling the test binary.
func runExporter(ctx context.Context, cfg config, log *logrus.Logger) error {
	cli, err := rpc.New(cfg.MoneroAddr, cfg.RPCUser, cfg.RPCPassword, log)
	if err != nil {
		return fmt.Errorf("rpc client: %w", err)
	}

	// An unreachable monerod must not terminate the exporter. monerod can be
	// mid-startup (loading the LMDB takes minutes), restarting, or briefly
	// unreachable, and the collector already encodes daemon health as metrics
	// (monero_up, monero_scrape_error) while Refresh re-runs on every scrape.
	// Exiting here would discard that signal and turn a transient blip into an
	// outage, so warn and keep serving; the exporter recovers on its own as soon
	// as the daemon answers.
	pingCtx, pingCancel := context.WithTimeout(ctx, 15*time.Second)
	if err := cli.Ping(pingCtx); err != nil {
		log.WithError(err).WithField("monero_addr", cfg.MoneroAddr).
			Warn("initial monerod RPC ping failed; serving metrics with monero_up 0 until the daemon recovers")
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
		// WithoutCancel, not Background: ctx is already cancelled by the time we
		// get here, so deriving from it directly would hand Shutdown an expired
		// context and abort the drain immediately.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
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
