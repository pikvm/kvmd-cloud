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
	"github.com/pikvm/kvmd-cloud/internal/hive"
	"github.com/pikvm/kvmd-cloud/internal/proxy"
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
		proxies, err := hive.Dial(ctx, group)
		if err != nil {
			return fmt.Errorf("unable to connect to hive: %w", err)
		}
		if proxies == nil || len(proxies.GetAvailableProxies()) == 0 {
			return fmt.Errorf("no proxies from hive")
		}
		for _, proxyInfo := range proxies.GetAvailableProxies() {
			group.Go(func() error {
				err := proxy.Dial(ctx, proxyInfo)
				if err != nil {
					return fmt.Errorf("unable to launch routing server: %w", err)
				}
				return fmt.Errorf("connection to proxy %s lost", proxyInfo.GetProxyEndpoint()) // TODO: remove on multi-proxy
			})
			break // TODO: support multiple
		}
		return nil
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
