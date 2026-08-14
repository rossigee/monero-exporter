package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rossigee/monero-exporter/internal/rpc"
	"github.com/sirupsen/logrus"
)

// runHealthCheck pings the configured monerod RPC endpoint and exits with a
// non-zero status if the daemon is unreachable. Used by container
// HEALTHCHECK directives.
func runHealthCheck(cfg config) error {
	log := logrus.New()
	log.SetOutput(io.Discard)

	cli, err := rpc.New(cfg.MoneroAddr, cfg.RPCUser, cfg.RPCPassword, log)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := cli.Ping(ctx); err != nil {
		return fmt.Errorf("monerod unreachable at %s: %w", cfg.MoneroAddr, err)
	}
	_, _ = fmt.Fprintf(os.Stdout, "monerod reachable at %s\n", cfg.MoneroAddr)
	return nil
}
