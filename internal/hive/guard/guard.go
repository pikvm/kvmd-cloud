package guard

import (
	"context"
	"errors"
	"time"

	hive_pb "github.com/pikvm/cloud-api/hive_for_agent"
	"github.com/pikvm/kvmd-cloud/internal/hive"
	"github.com/pikvm/kvmd-cloud/internal/proxy"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	attemptInterval = 3 * time.Second
)

var shutdownRequested = errors.New("")

func Guard(ctx context.Context) error {
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		attempt++
		if attempt > 1 {
			time.Sleep(attemptInterval)
		}
		hiveConnection, err := hive.Dial(ctx)
		if err != nil {
			log.WithError(err).Errorf("unable to connect to hive")
			continue
		}
		hiveConnErrChan := make(chan error)
		go func() {
			hiveConnErrChan <- hive.ProcessEvents(hiveConnection)
			close(hiveConnErrChan)
			log.Debugf("connection to hive %s lost", hiveConnection.Addr)
		}()
		proxies, err := hiveConnection.HiveClient.GetAvailableProxies(ctx, &emptypb.Empty{})
		if err != nil {
			log.WithError(err).Errorf("unable to get proxy list")
			hiveConnection.GrpcConn.Close()
			continue
		}
		if len(proxies.GetAvailableProxies()) < 1 {
			log.Errorf("proxy list is empty")
			hiveConnection.GrpcConn.Close()
			continue
		}
		proxyInfo := proxies.GetAvailableProxies()[0] // TODO: support multiple

		proxiesCtx, cancelProxies := context.WithCancel(ctx)
		// proxyConnErrChan := make(chan error)
		go ProxyGuard(proxiesCtx, proxyInfo)

		select {
		case <-ctx.Done():
			// shutdown
			hiveConnection.GrpcConn.Close()
			cancelProxies()
			return nil
		case err = <-hiveConnErrChan:
			log.WithError(err).Errorf("error in hive connection")
			hiveConnection.GrpcConn.Close()
			cancelProxies()
			continue
			// case <-proxyConnErrChan:
			// 	log.WithError(err).Errorf("error in proxy connection")
			// 	hiveConnection.GrpcConn.Close()
			// 	continue
		}
	}
}

func ProxyGuard(ctx context.Context, proxyInfo *hive_pb.AvailableProxies_ProxyInfo) {
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			// shut down
			return
		default:
		}
		attempt++
		if attempt > 1 {
			time.Sleep(attemptInterval)
		}
		proxyConnection, err := proxy.Dial(ctx, proxyInfo)
		if err != nil {
			log.WithError(err).Errorf("unable to connect to proxy %s", proxyInfo.GetProxyEndpoint())
			continue
		}
		if err = proxy.ProcessEvents(proxyConnection); err != nil {
			log.WithError(err).Errorf("proxy connection error")
			proxyConnection.GrpcConn.Close()
		}
	}
}
