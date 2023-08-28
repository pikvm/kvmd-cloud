package guard

import (
	"context"
	"errors"
	"time"

	hive_pb "github.com/pikvm/cloud-api/proto/hive"
	"github.com/pikvm/kvmd-cloud/internal/hive"
	"github.com/pikvm/kvmd-cloud/internal/proxy"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	attemptInterval = 3 * time.Second
)

func Guard(ctx context.Context) error {
	var sleepTime time.Duration
	attempt := 0
	veryFirstTime := true
	for {
		attempt++
		if attempt > 1 {
			sleepTime = attemptInterval
		} else {
			sleepTime = time.Millisecond
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(sleepTime):
		}

		if !veryFirstTime {
			logrus.Debugf("trying to re-connect to hive (attempt %d)", attempt)
		} else {
			veryFirstTime = false
		}
		hiveConnection, err := hive.Dial(ctx)
		if err != nil {
			logrus.WithError(err).Errorf("unable to connect to hive")
			continue
		}
		attempt = 0

		proxies, err := hiveConnection.Client.GetAvailableProxies(ctx, &emptypb.Empty{})
		if err != nil {
			logrus.WithError(err).Errorf("unable to get proxy list")
			hiveConnection.Rpc.Close()
			continue
		}
		if len(proxies.GetAvailableProxies()) < 1 {
			logrus.Error("proxy list is empty")
			hiveConnection.Rpc.Close()
			continue
		}
		proxyInfo := proxies.GetAvailableProxies()[0] // TODO: support multiple
		hiveCtx := hiveConnection.Context()
		go ProxyGuard(hiveCtx, proxyInfo)

		<-hiveCtx.Done()
		select {
		case <-ctx.Done():
			// Shutdown
			return nil
		default:
		}

		err = context.Cause(hiveCtx)
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.WithError(err).Errorf("error in hive connection")
		}
	}
}

func ProxyGuard(ctx context.Context, proxyInfo *hive_pb.AvailableProxies_ProxyInfo) {
	var sleepTime time.Duration
	attempt := 0
	veryFirstTime := true
	for {
		attempt++
		if attempt > 1 {
			sleepTime = attemptInterval
		} else {
			sleepTime = time.Millisecond
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(sleepTime):
		}

		endpoint := proxyInfo.GetProxyEndpoint()

		if !veryFirstTime {
			logrus.Debugf("trying to re-connect to proxy %s (attempt %d)", endpoint, attempt)
		} else {
			veryFirstTime = false
		}

		proxyConnection, err := proxy.Dial(ctx, proxyInfo)
		if err != nil {
			logrus.WithError(err).Errorf("unable to connect to proxy %s", endpoint)
			continue
		}
		proxyCtx := proxyConnection.Context()
		attempt = 0

		<-proxyCtx.Done()
		select {
		case <-ctx.Done():
			// hive went away or shutdown has been requested
			return
		default:
		}

		err = context.Cause(proxyCtx)
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.WithError(err).Errorf("error in proxy connection %s", endpoint)
		}
	}
}
