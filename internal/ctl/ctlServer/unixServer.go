package ctlserver

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/ctl"
	log "github.com/sirupsen/logrus"
	ginlogrus "github.com/toorop/gin-logrus"
)

func RunServer(ctx context.Context) error {
	if log.GetLevel() < log.DebugLevel {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(ginlogrus.Logger(log.StandardLogger()), gin.Recovery())
	setupRoutes(r)

	srv := &http.Server{
		Handler: r,
	}
	log.Warn(config.Cfg.UnixCtlSocket)
	unixListener, err := net.Listen("unix", config.Cfg.UnixCtlSocket)
	if err != nil {
		return err
	}

	var serveStopError error
	runErrorChan := make(chan error)
	go func() {
		log.Info("Listening on unix socket ", config.Cfg.UnixCtlSocket)
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
		log.Info("UNIX socket server requested to stop. Trying to do it gracefully")
		graceCtx, graceCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer graceCancel()
		if err := srv.Shutdown(graceCtx); err != nil {
			log.WithError(err).Error("UNIX socket server graceful shutdown error")
			return err
		}

		select {
		case <-runErrorChan:
		default:
		}
	}
	if serveStopError == http.ErrServerClosed {
		serveStopError = nil
		log.Info("UNIX socket server stopped")
	} else {
		log.WithError(serveStopError).Error("UNIX socket server stopped")
	}

	return serveStopError
}

func setupRoutes(r *gin.Engine) {
	r.GET("/status", getStatus)
	r.POST("/certbotAdd", certbotAdd)
	r.POST("/certbotDel", certbotDel)
}

func getStatus(c *gin.Context) {
	c.JSON(200, ctl.ApplicationStatusResponse{
		PingerField: "Yahoo!!",
	})
}
