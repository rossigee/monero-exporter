package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
