package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNewRejectsMissingScheme(t *testing.T) {
	if _, err := New("localhost:18089", "", "", logrus.New()); err == nil {
		t.Fatal("expected error for missing scheme")
	}
}

func newTestServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cli, err := New(srv.URL, "", "", logrus.New())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return cli
}

func TestGetInfo(t *testing.T) {
	cli := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/json_rpc" {
			t.Errorf("path = %q, want /json_rpc", got)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if req["method"] != "get_info" {
			t.Errorf("method = %v, want get_info", req["method"])
		}
		_, _ = w.Write([]byte(`{"result":{"height":3215600,"target_height":3215600,"tx_pool_size":12,"synchronized":true,"mainnet":true,"restricted":true,"start_time":1700000000,"database_size":12345678,"free_space":1000000000}}`))
	})

	info, err := cli.GetInfo(context.Background())
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if info.Height != 3215600 {
		t.Errorf("height = %d, want 3215600", info.Height)
	}
	if !info.Synchronized {
		t.Error("expected synchronized")
	}
	if info.TxPoolSize != 12 {
		t.Errorf("tx pool = %d, want 12", info.TxPoolSize)
	}
}

func TestGetInfoRPCError(t *testing.T) {
	cli := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"code":-1,"message":"invalid method"}}`))
	})

	if _, err := cli.GetInfo(context.Background()); err == nil {
		t.Fatal("expected error from RPC error payload")
	} else if !strings.Contains(err.Error(), "invalid method") {
		t.Errorf("error = %q, want mention of message", err)
	}
}

func TestGetInfoUnauthorized(t *testing.T) {
	cli := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	if _, err := cli.GetInfo(context.Background()); err == nil {
		t.Fatal("expected 401 error")
	} else if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %q, want 401 mention", err)
	}
}

func TestBasicAuthSent(t *testing.T) {
	cli := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok {
			t.Error("expected basic auth header")
		}
		if u != "user" || p != "pass" {
			t.Errorf("basic auth = %q/%q, want user/pass", u, p)
		}
		_, _ = w.Write([]byte(`{"result":{}}`))
	})
	cli.basicUser = "user"
	cli.basicPass = "pass"

	if _, err := cli.GetInfo(context.Background()); err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
}

func TestGetLastBlockHeader(t *testing.T) {
	cli := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"block_header":{"major_version":12,"minor_version":12,"height":3215600,"timestamp":1700000000,"difficulty":250000000000,"reward":600000000,"num_txes":3}}}`))
	})

	h, err := cli.GetLastBlockHeader(context.Background())
	if err != nil {
		t.Fatalf("GetLastBlockHeader: %v", err)
	}
	if h.Height != 3215600 {
		t.Errorf("height = %d, want 3215600", h.Height)
	}
	if h.Reward != 600000000 {
		t.Errorf("reward = %d, want 600000000", h.Reward)
	}
	if h.TxCount != 3 {
		t.Errorf("tx count = %d, want 3", h.TxCount)
	}
}

func TestGetBlockCount(t *testing.T) {
	cli := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"count":3215600}}`))
	})

	count, err := cli.GetBlockCount(context.Background())
	if err != nil {
		t.Fatalf("GetBlockCount: %v", err)
	}
	if count != 3215600 {
		t.Errorf("count = %d, want 3215600", count)
	}
}

func TestHexToDec(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
	}{
		{"", 0},
		{"1234", 0x1234},
		{"0x10", 0},
		{"zz", 0},
	}
	for _, c := range cases {
		if got := HexToDec(c.in); got != c.want {
			t.Errorf("HexToDec(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
