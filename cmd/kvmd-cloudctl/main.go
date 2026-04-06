package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/pikvm/kvmd-cloud/cmd/kvmd-cloudctl/ctl_client"
	"github.com/pikvm/kvmd-cloud/cmd/kvmd-cloudctl/setup"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	config.InitBootstrapLogger()

	rootCmd := &cli.Command{
		Flags:                  config.GetGlobalFlags(),
		UseShortOptionHandling: true,
		Commands:               subCommands(),
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			config.LoadConfig(cmd)
			return ctx, nil
		},
	}

	if err := rootCmd.Run(ctx, os.Args); err != nil {
		log.Fatal().Err(err).Msg("Failed to run command")
		return
	}
}

func subCommands() []*cli.Command {
	return []*cli.Command{
		ctl_client.BuildStatusCommand(),
		setup.BuildCommand(),
	}
}
