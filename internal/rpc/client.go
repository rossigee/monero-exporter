// Package rpc issues JSON-RPC calls against a monerod instance.
//
// Only endpoints that the *restricted* RPC exposes are implemented:
//   - get_info
//   - get_block_count
//   - get_last_block_header
//
// Exposing this code is enough to power a Prometheus scrape without
// requiring the unrestricted admin RPC.
package rpc

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Client issues monerod JSON-RPC calls over HTTP(S).
type Client struct {
	log *logrus.Logger

	basicUser string
	basicPass string

	baseURL string
}

// New returns a Client speaking to baseURL (e.g. http://localhost:18089).
//
// If user and pass are both non-empty, HTTP basic auth is added to every
// request.
func New(baseURL, user, pass string, log *logrus.Logger) (*Client, error) {
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		return nil, fmt.Errorf("monero-addr must include scheme, got %q", baseURL)
	}
	if user != "" && pass == "" || user == "" && pass != "" {
		log.Warnf("rpc-user or rpc-password set without the other; sending unauthenticated requests")
	}
	return &Client{
		log:       log,
		baseURL:   baseURL,
		basicUser: user,
		basicPass: pass,
	}, nil
}

// Ping issues a cheap RPC to confirm the daemon is up.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.GetInfo(ctx)
	return err
}

// GetInfo calls /json_rpc get_info and returns the parsed payload.
func (c *Client) GetInfo(ctx context.Context) (*GetInfoResult, error) {
	var res GetInfoResult
	if err := c.call(ctx, "get_info", &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// GetBlockCount calls /json_rpc get_block_count.
func (c *Client) GetBlockCount(ctx context.Context) (uint64, error) {
	var res struct {
		Count uint64 `json:"count"`
	}
	if err := c.call(ctx, "get_block_count", &res); err != nil {
		return 0, err
	}
	return res.Count, nil
}

// GetLastBlockHeader calls /json_rpc get_last_block_header.
func (c *Client) GetLastBlockHeader(ctx context.Context) (*BlockHeader, error) {
	var res struct {
		Header BlockHeader `json:"block_header"`
	}
	if err := c.call(ctx, "get_last_block_header", &res); err != nil {
		return nil, err
	}
	return &res.Header, nil
}

// call invokes method via JSON-RPC and decodes the inner `result` into out.
func (c *Client) call(ctx context.Context, method string, out any) error {
	type rpcResponse struct {
		ID      string          `json:"id"`
		JSONRPC string          `json:"jsonrpc"`
		Result  json.RawMessage `json:"result"`
		Error   *rpcError       `json:"error"`
	}

	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      "0",
		"method":  method,
	}
	if c.basicUser != "" {
		body["params"] = map[string]any{
			"username": c.basicUser,
			"password": c.basicPass,
		}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/json_rpc", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.basicUser != "" {
		httpReq.SetBasicAuth(c.basicUser, c.basicPass)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("rpc %s returned 401 (auth required; provide rpc-user and rpc-password)", method)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rpc %s returned status %d", method, resp.StatusCode)
	}

	var rpcRes rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcRes); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if rpcRes.Error != nil {
		return fmt.Errorf("rpc %s error: %s", method, rpcRes.Error.Message)
	}
	if err := json.Unmarshal(rpcRes.Result, out); err != nil {
		return fmt.Errorf("decode result: %w", err)
	}
	return nil
}

// rpcError matches monerod's JSON-RPC error envelope.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// GetInfoResult contains the subset of fields we consume from
// get_info. Most fields are exposed as raw json.RawMessage so we
// don't need to track every monerod addition.
type GetInfoResult struct {
	Height               uint64 `json:"height"`
	TargetHeight         uint64 `json:"target_height"`
	TopHash              string `json:"top_hash"`
	TopBlockDifficulty   uint64 `json:"difficulty"`
	TxPoolSize           uint64 `json:"tx_pool_size"`
	CumulativeDifficulty uint64 `json:"cumulative_difficulty"`
	BlockSizeLimit       uint64 `json:"block_size_limit"`
	BlockSizeMedian      uint64 `json:"block_size_median"`
	Offline              bool   `json:"offline"`
	Synchronized         bool   `json:"synchronized"`
	Mainnet              bool   `json:"mainnet"`
	Version              uint64 `json:"version"`
	IncomingConnections  uint64 `json:"incoming_connections_count"`
	OutgoingConnections  uint64 `json:"outgoing_connections_count"`
	RPCConnections       uint64 `json:"rpc_connections_count"`
	StartTime            int64  `json:"start_time"`
	FreeSpace            uint64 `json:"free_space"`
	DatabaseSize         uint64 `json:"database_size"`
	NetRxBytes           uint64 `json:"cumulative_difficulty_total"`
	AlternativeBlocks    uint64 `json:"alt_blocks_count"`
	Restricted           bool   `json:"restricted"`
	BootstrapDaemonAddr  string `json:"bootstrap_daemon_address"`
	UpdateAvailable      bool   `json:"update_available"`
	DBFree               bool   `json:"database_available"`
}

// BlockHeader is a minimal subset of /json_rpc get_last_block_header.block_header.
type BlockHeader struct {
	MajorVersion uint8  `json:"major_version"`
	MinorVersion uint8  `json:"minor_version"`
	Height       uint64 `json:"height"`
	Timestamp    uint64 `json:"timestamp"`
	Difficulty   uint64 `json:"difficulty"`
	Reward       uint64 `json:"reward"`
	Hash         string `json:"hash"`
	TxCount      uint64 `json:"num_txes"`
}

// HexToDec safely converts a hex string to an integer; returns 0 for empty.
func HexToDec(s string) uint64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return 0
	}
	var n uint64
	for _, by := range b {
		n = n<<8 | uint64(by)
	}
	return n
}
