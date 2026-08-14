package collector

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/rossigee/monero-exporter/internal/rpc"
)

const infoPayload = `{"result":{"height":3215600,"target_height":3215600,"tx_pool_size":12,"offline":false,"synchronized":true,"mainnet":true,"restricted":true,"incoming_connections_count":3,"outgoing_connections_count":2,"rpc_connections_count":1,"database_size":12345678,"free_space":1000000000,"start_time":1700000000}}`

const headerPayload = `{"result":{"block_header":{"major_version":12,"minor_version":12,"height":3215600,"timestamp":1700000000,"difficulty":250000000000,"reward":600000000,"num_txes":3}}}`

func newMockMonerod(t *testing.T) *rpc.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "get_info") {
			_, _ = w.Write([]byte(infoPayload))
			return
		}
		if strings.Contains(string(body), "get_last_block_header") {
			_, _ = w.Write([]byte(headerPayload))
			return
		}
		_, _ = w.Write([]byte(`{"error":{"code":-1,"message":"unknown method"}}`))
	}))
	t.Cleanup(srv.Close)

	cli, err := rpc.New(srv.URL, "", "", logrus.New())
	if err != nil {
		t.Fatalf("rpc.New: %v", err)
	}
	return cli
}

func TestCollectEmittedMetrics(t *testing.T) {
	cli := newMockMonerod(t)
	col := New(cli, logrus.New())

	col.Refresh(context.Background())

	registry := prometheus.NewRegistry()
	if err := registry.Register(col); err != nil {
		t.Fatalf("register: %v", err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	got := map[string]bool{}
	for _, f := range families {
		got[f.GetName()] = true
	}

	want := []string{
		"monero_up",
		"monero_scrape_error",
		"monero_scrape_timestamp_seconds",
		"monero_info_height",
		"monero_info_target_height",
		"monero_info_tx_pool_size",
		"monero_info_offline",
		"monero_info_synchronized",
		"monero_info_mainnet",
		"monero_info_restricted",
		"monero_info_incoming_connections",
		"monero_info_outgoing_connections",
		"monero_info_rpc_connections",
		"monero_info_database_size_bytes",
		"monero_info_free_space_bytes",
		"monero_info_start_time_seconds",
		"monero_info_uptime_seconds",
		"monero_lastblock_height",
		"monero_lastblock_difficulty",
		"monero_lastblock_reward",
		"monero_lastblock_major_version",
		"monero_lastblock_minor_version",
		"monero_lastblock_timestamp_seconds",
		"monero_lastblock_transactions",
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("metric %q not emitted", name)
		}
	}

	// Verify a couple of values.
	for _, f := range families {
		switch f.GetName() {
		case "monero_info_height":
			if v := f.GetMetric()[0].GetGauge().GetValue(); v != 3215600 {
				t.Errorf("height = %v, want 3215600", v)
			}
		case "monero_up":
			if v := f.GetMetric()[0].GetGauge().GetValue(); v != 1 {
				t.Errorf("monero_up = %v, want 1", v)
			}
		case "monero_lastblock_reward":
			if v := f.GetMetric()[0].GetGauge().GetValue(); v != 600000000 {
				t.Errorf("reward = %v, want 600000000", v)
			}
		}
	}
}

func TestCollectAfterFailure(t *testing.T) {
	cli := newMockMonerod(t)
	col := New(cli, logrus.New())

	// Point the client at a dead endpoint so Refresh fails.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(dead.Close)
	col.client, _ = rpc.New(dead.URL, "", "", logrus.New())

	col.Refresh(context.Background())

	registry := prometheus.NewRegistry()
	if err := registry.Register(col); err != nil {
		t.Fatalf("register: %v", err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	for _, f := range families {
		switch f.GetName() {
		case "monero_up":
			if v := f.GetMetric()[0].GetGauge().GetValue(); v != 0 {
				t.Errorf("monero_up = %v, want 0 on failure", v)
			}
		case "monero_scrape_error":
			if v := f.GetMetric()[0].GetGauge().GetValue(); v != 1 {
				t.Errorf("monero_scrape_error = %v, want 1", v)
			}
		}
	}
}
