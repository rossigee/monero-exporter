package main

import (
	"github.com/spf13/cobra"
)

// config holds the runtime configuration of the exporter.
type config struct {
	BindAddr      string
	TelemetryPath string
	MoneroAddr    string
	RPCUser       string
	RPCPassword   string
	LogLevel      string
}

// loadConfig extracts configuration from the cobra command's flags.
func loadConfig(cmd *cobra.Command) (config, error) {
	bind, err := cmd.Flags().GetString("bind-addr")
	if err != nil {
		return config{}, err
	}
	tp, err := cmd.Flags().GetString("telemetry-path")
	if err != nil {
		return config{}, err
	}
	addr, err := cmd.Flags().GetString("monero-addr")
	if err != nil {
		return config{}, err
	}
	user, err := cmd.Flags().GetString("rpc-user")
	if err != nil {
		return config{}, err
	}
	pass, err := cmd.Flags().GetString("rpc-password")
	if err != nil {
		return config{}, err
	}
	lvl, err := cmd.Flags().GetString("log-level")
	if err != nil {
		return config{}, err
	}
	return config{
		BindAddr:      bind,
		TelemetryPath: tp,
		MoneroAddr:    addr,
		RPCUser:       user,
		RPCPassword:   pass,
		LogLevel:      lvl,
	}, nil
}
