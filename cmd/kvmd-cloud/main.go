package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/pikvm/kvmd-cloud/cmd/kvmd-cloud/ctl_server"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/config/vars"
	"github.com/pikvm/kvmd-cloud/internal/proxy"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

func root(ctx context.Context, rootCmd *cli.Command) error {
	logger := log.Logger

	if rootCmd.Bool("version") {
		println(vars.VersionString)
		return nil
	}

	if !rootCmd.Bool("run") {
		logger.Error().Msg("Forgot to specify --run flag?")
		cli.ShowRootCommandHelpAndExit(rootCmd, 1)
	}

	logger.Info().Str("version", vars.VersionString).Msgf("Starting %s", vars.AppName)

	group, ctx := errgroup.WithContext(ctx)

	proxyPool := proxy.NewProxyPool()
	group.Go(func() error {
		proxyPool.Serve(ctx)
		return nil
	})

	group.Go(func() error {
		err := ctl_server.RunServer(ctx)
		if err != nil {
			err = fmt.Errorf("unable to launch ctl server: %w", err)
		}
		return err
	})

	return group.Wait()
}

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	config.InitBootstrapLogger()

	rootCmd := &cli.Command{
		Usage:                  "PiKVM Cloud Agent",
		Flags:                  config.GetGlobalFlags(),
		UseShortOptionHandling: true,
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			config.LoadConfig(cmd)
			return ctx, nil
		},
		Action: root,
	}

	if err := rootCmd.Run(ctx, os.Args); err != nil {
		log.Fatal().Err(err).Msg("Error running kvmd-cloud")
	}
}
