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
	"github.com/pikvm/kvmd-cloud/internal/hive/guard"
)

func root(rootCmd *cobra.Command, args []string) error {
	group, ctx := errgroup.WithContext(rootCmd.Context())

	if ok, err := rootCmd.Flags().GetBool("run"); err != nil {
		return err
	} else if !ok {
		fmt.Fprintf(log.StandardLogger().Out, "Forgot '--run'?\n\n")
		if err := rootCmd.Usage(); err != nil {
			return err
		}
		return nil
	}

	group.Go(func() error {
		err := ctlserver.RunServer(ctx)
		if err != nil {
			err = fmt.Errorf("unable to launch ctl server: %w", err)
		}
		return err
	})

	group.Go(func() error {
		err := guard.Guard(ctx)
		if err != nil {
			err = fmt.Errorf("unable to launch hive & proxies connections: %w", err)
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
