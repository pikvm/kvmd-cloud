package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/config/vars"
	"github.com/sirupsen/logrus"
)

func main() {
	var err error
	defer func() {
		if !vars.Debug {
			if panicErr := recover(); panicErr != nil {
				logrus.Error(panicErr)
				os.Exit(1)
			}
		}

		if err != nil {
			logrus.Error(err)
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

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			os.Exit(1)
		case <-done:
			return
		}
	}()
	err = rootCmd.ExecuteContext(ctx)
}
