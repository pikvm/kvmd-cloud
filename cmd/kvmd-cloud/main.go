package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/config/vars"
	ctlserver "github.com/pikvm/kvmd-cloud/internal/ctl/ctlServer"
	"github.com/pikvm/kvmd-cloud/internal/routing"
)

func root(rootCmd *cobra.Command, args []string) error {
	group, ctx := errgroup.WithContext(rootCmd.Context())

	group.Go(func() error {
		err := ctlserver.RunServer(ctx)
		if err != nil {
			err = fmt.Errorf("unable to launch ctl server: %w", err)
		}
		return err
	})
	group.Go(func() error {
		err := routing.Serve(ctx)
		if err != nil {
			err = fmt.Errorf("unable to launch routing server: %w", err)
		}
		return err
	})

	return group.Wait()
}

func main() {
	var err error
	defer func() {
		if !vars.Debug {
			if panicErr := recover(); panicErr != nil {
				log.Error(panicErr)
				os.Exit(1)
			}
		}

		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
	}()

	rootCmd, err := config.Initialize(buildCobra)
	if err != nil {
		return
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt, syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	err = rootCmd.ExecuteContext(ctx)
}
