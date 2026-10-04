package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rossigee/monero-exporter/internal/collector"
	"github.com/rossigee/monero-exporter/internal/rpc"
	"github.com/sirupsen/logrus"
)

func newMuxWithMock(t *testing.T) *http.ServeMux {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "get_info") {
			_, _ = w.Write([]byte(`{"result":{"height":3215600,"synchronized":true,"mainnet":true,"restricted":true,"start_time":1700000000,"database_size":12345678}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{"block_header":{"major_version":12,"height":3215600,"timestamp":1700000000,"difficulty":250000000000,"reward":600000000,"num_txes":3}}}`))
	}))
	t.Cleanup(srv.Close)

	cli, err := rpc.New(srv.URL, "", "", logrus.New())
	if err != nil {
		t.Fatalf("rpc.New: %v", err)
	}
	col := collector.New(cli, logrus.New())

	registry := prometheus.NewRegistry()
	if err := registry.Register(col); err != nil {
		t.Fatalf("registry.Register: %v", err)
	}
	return newMux(col, "/metrics", registry)
}

func TestMetricsEndpoint(t *testing.T) {
	mux := newMuxWithMock(t)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"monero_up 1",
		"monero_info_height",
		"monero_lastblock_reward",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q", want)
		}
	}
}

func TestMetricsEndpointRefreshes(t *testing.T) {
	mux := newMuxWithMock(t)

	// First scrape warms the cache, second must serve 200 again
	// (the handler refreshes before every scrape).
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("scrape %d status = %d, want 200", i, rec.Code)
		}
	}
}

func TestHealthzEndpoint(t *testing.T) {
	mux := newMuxWithMock(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "ok" {
		t.Errorf("body = %q, want ok", body)
	}
}

// deadAddr returns an address on the loopback range that nothing is listening
// on, so RPC calls to it fail fast with connection refused.
func deadAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return addr
}

// TestRunExporterSurvivesUnreachableMonerod is the regression test for the
// startup crash loop: monerod that is still loading its LMDB, restarting, or
// briefly unreachable must not terminate the exporter. Before the fix the
// startup ping returned an error, runExporter propagated it, and the process
// exited 1 on every restart.
func TestRunExporterSurvivesUnreachableMonerod(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	bindAddr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	cfg := config{
		BindAddr:      bindAddr,
		TelemetryPath: "/metrics",
		MoneroAddr:    "http://" + deadAddr(t),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := logrus.New()
	log.SetOutput(io.Discard)

	errCh := make(chan error, 1)
	go func() { errCh <- runExporter(ctx, cfg, log) }()

	// The exporter must come up and answer /metrics despite the dead daemon.
	var body string
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := http.Get("http://" + bindAddr + "/metrics")
		if err == nil {
			raw, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			body = string(raw)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("exporter never served /metrics: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Daemon health is reported as metrics, not as a dead process.
	for _, want := range []string{"monero_up 0", "monero_scrape_error 1"} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q", want)
		}
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runExporter returned %v, want clean shutdown", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("runExporter did not shut down after context cancellation")
	}
}
