// Package main is the entry point for the monero-exporter Prometheus exporter.
//
// monero-exporter polls a monerod instance via its JSON-RPC API and exposes the
// resulting state to Prometheus.
//
// It works in concert with a typical Prometheus + Grafana stack:
//
//	[grafana] --> queries --> [prometheus] -- scrape /metrics --> [monero-exporter]
//	                                                                   |
//	                                                                HTTP+JSON
//	                                                                   |
//	                                                                  [monerod]
package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	log := logrus.New()
	log.SetFormatter(&prefixed.TextFormatter{
		FullTimestamp: true,
	})
	log.SetLevel(logrus.InfoLevel)

	cmd := newRootCmd(log)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newRootCmd returns the root cobra command for monero-exporter.
func newRootCmd(log *logrus.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monero-exporter",
		Short: "Prometheus exporter for monerod",
		Long: `monero-exporter queries monerod's JSON-RPC API and exposes the
resulting state on a Prometheus scrape endpoint.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if show, _ := cmd.Flags().GetBool("show-version"); show {
				fmt.Printf("monero-exporter %s (commit %s, built %s)\n", version, commit, date)
				return nil
			}
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			if hc, _ := cmd.Flags().GetBool("health-check"); hc {
				return runHealthCheck(cfg)
			}
			lvl, err := logrus.ParseLevel(cfg.LogLevel)
			if err != nil {
				return fmt.Errorf("invalid log-level %q: %w", cfg.LogLevel, err)
			}
			log.SetLevel(lvl)
			return runExporter(cfg, log)
		},
	}

	cmd.Flags().String("bind-addr", ":9000", "address to bind the prometheus exporter to")
	cmd.Flags().String("telemetry-path", "/metrics", "path at which metrics are served")
	cmd.Flags().String("monero-addr", "http://localhost:18089", "JSON-RPC base URL of the monerod daemon (default uses the restricted RPC port)")
	cmd.Flags().String("rpc-user", "", "monerod RPC basic auth username (leave empty for unrestricted local RPC)")
	cmd.Flags().String("rpc-password", "", "monerod RPC basic auth password")
	cmd.Flags().String("log-level", "info", "log level (trace, debug, info, warn, error)")
	cmd.Flags().Bool("show-version", false, "print version and exit")
	cmd.Flags().Bool("health-check", false, "ping the monerod RPC endpoint and exit 0 on success (for container healthchecks)")

	cmd.AddCommand(newVersionCmd())

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information and exit",
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Printf("monero-exporter %s (commit %s, built %s)\n", version, commit, date)
			return nil
		},
	}
}
