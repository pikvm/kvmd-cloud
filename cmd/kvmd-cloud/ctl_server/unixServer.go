package ctl_server

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pikvm/kvmd-cloud/cmd/kvmd-cloud/ctl_server/status"
	"github.com/pikvm/kvmd-cloud/internal/agent"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/sirupsen/logrus"
	ginlogrus "github.com/toorop/gin-logrus"
)

func setupRoutes(r *gin.Engine) {
	status.SetupRoutes(r)
	// ...
}

func RunServer(ctx context.Context, agent *agent.Agent) error {
	if logrus.GetLevel() < logrus.DebugLevel {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(ginlogrus.Logger(logrus.StandardLogger()), gin.Recovery())
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
		logrus.Info("Listening on unix socket ", config.Cfg.UnixCtlSocket)
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
		logrus.Info("UNIX socket server requested to stop. Trying to do it gracefully")
		graceCtx, graceCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer graceCancel()
		if err := srv.Shutdown(graceCtx); err != nil {
			logrus.WithError(err).Error("UNIX socket server graceful shutdown error")
			return err
		}

		select {
		case <-runErrorChan:
		default:
		}
	}
	if serveStopError == http.ErrServerClosed {
		serveStopError = nil
		logrus.Info("UNIX socket server stopped")
	} else {
		logrus.WithError(serveStopError).Error("UNIX socket server stopped")
	}

	return serveStopError
}
