package main

import (
	"log/slog"

	"github.com/hanzoai/o11y/cmd"
	"github.com/hanzoai/o11y/pkg/instrumentation"
)

func main() {
	// initialize logger for logging in the cmd/ package. This logger is different from the logger used in the application.
	logger := instrumentation.NewLogger(instrumentation.Config{Logs: instrumentation.LogsConfig{Level: slog.LevelInfo}})

	// register a list of commands to the root command
	registerServer(cmd.RootCmd, logger)
	cmd.RegisterGenerate(cmd.RootCmd, logger)

	cmd.Execute(logger)
}
