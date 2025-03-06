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

	"github.com/pikvm/kvmd-cloud/cmd/kvmd-cloud/ctl_server"
	"github.com/pikvm/kvmd-cloud/internal/agent"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/config/vars"
)

func root(rootCmd *cobra.Command, args []string) error {
	group, ctx := errgroup.WithContext(rootCmd.Context())

	if ok, err := rootCmd.Flags().GetBool("version"); err != nil {
		return err
	} else if ok {
		println(vars.VersionString)
		return nil
	}

	if ok, err := rootCmd.Flags().GetBool("run"); err != nil {
		return err
	} else if !ok {
		fmt.Fprintf(log.StandardLogger().Out, "Forgot '--run'?\n\n")
		if err := rootCmd.Usage(); err != nil {
			return err
		}
		return nil
	}

	agent := agent.NewAgent()

	group.Go(func() error {
		err := agent.Run(ctx)
		if err != nil {
			err = fmt.Errorf("unable to launch hive & proxies connections: %w", err)
		}
		return err
	})

	group.Go(func() error {
		err := ctl_server.RunServer(ctx, agent)
		if err != nil {
			err = fmt.Errorf("unable to launch ctl server: %w", err)
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
