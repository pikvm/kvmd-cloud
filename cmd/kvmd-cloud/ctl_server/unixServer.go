package ctl_server

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pikvm/kvmd-cloud/cmd/kvmd-cloud/ctl_server/status"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func setupRoutes(r *gin.Engine) {
	status.SetupRoutes(r)
	// ...
}

func RunServer(ctx context.Context) error {
	logger := log.Logger

	if zerolog.GlobalLevel() > zerolog.DebugLevel {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	setupRoutes(r)

	srv := &http.Server{
		Handler: r,
	}
	unixListener, err := net.Listen("unix", config.Cfg.UnixCtlSocket)
	if err != nil {
		return err
	}

	var serveStopError error
	runErrorChan := make(chan error)
	go func() {
		logger.Info().Msg("Listening on unix socket " + config.Cfg.UnixCtlSocket)
		serveStopError = srv.Serve(unixListener)
		runErrorChan <- serveStopError
		close(runErrorChan)
	}()

	stopRequested := false
	select {
	case <-ctx.Done():
		stopRequested = true
	case <-runErrorChan:
	}

	if stopRequested {
		logger.Info().Msg("UNIX socket server requested to stop. Trying to do it gracefully")
		graceCtx, graceCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer graceCancel()
		if err := srv.Shutdown(graceCtx); err != nil {
			logger.Err(err).Msg("UNIX socket server graceful shutdown error")
			return err
		}

		select {
		case <-runErrorChan:
		default:
		}
	}
	if serveStopError == http.ErrServerClosed {
		serveStopError = nil
	}
	logger.Err(serveStopError).Msg("UNIX socket server stopped")

	return serveStopError
}
