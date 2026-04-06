package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/rand"
	"os"
	"sync/atomic"
	"time"

	proxyagent_pb "github.com/pikvm/cloud-api/proto/proxyagent"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/config/vars"
	"github.com/rs/zerolog"
	"github.com/xornet-sl/go-xrpc/xrpc"
	"google.golang.org/grpc/metadata"
)

type ProxyConnection struct {
	Addr   string
	rpc    atomic.Value // *xrpc.RpcConn
	cancel context.CancelFunc
}

func (this *ProxyConnection) GetRpcConn() *xrpc.RpcConn {
	return this.rpc.Load().(*xrpc.RpcConn)
}

func (this *ProxyConnection) IsReady() bool {
	return this.GetRpcConn() != nil
}

func (this *ProxyConnection) Close() {
	this.cancel()
}

func loadTLSCredentials() (*tls.Config, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	if config.Cfg.SSL.Ca != "" {
		caCert, err := os.ReadFile(config.Cfg.SSL.Ca)
		if err != nil {
			return nil, err
		}
		certPool.AppendCertsFromPEM(caCert)
	}
	config := &tls.Config{
		RootCAs: certPool,
	}
	return config, nil
}

func ConnectWithRetry(ctx context.Context, proxyEndpoint string) (*ProxyConnection, error) {
	logger := zerolog.Ctx(ctx).With().Fields(map[string]any{
		"component":      "proxy",
		"proxy_endpoint": proxyEndpoint,
	}).Logger()
	ctx = logger.WithContext(ctx)

	ctx, cancel := context.WithCancel(ctx)

	proxyConnection := &ProxyConnection{
		Addr:   proxyEndpoint,
		rpc:    atomic.Value{},
		cancel: cancel,
	}

	onOpen := func(connCtx context.Context, conn *xrpc.RpcConn) (context.Context, error) {
		logger.Info().Msg("connected to proxy")
		proxyConnection.rpc.Store(conn)
		return nil, nil
	}

	onClosed := func(connCtx context.Context, conn *xrpc.RpcConn, closeError error) {
		if ctx.Err() == nil {
			logger.Err(closeError).Msg("connection to proxy lost, retrying...")
		} else {
			logger.Info().Msg("connection to proxy closed")
		}
	}

	opts := []xrpc.Option{
		xrpc.WithOnLogCallback(onLog),
		xrpc.WithOnDebugLogCallback(onDebugLog),
		xrpc.WithConnOpenCallback(onOpen),
		xrpc.WithConnClosedCallback(onClosed),
	}

	if !config.Cfg.NoSSL {
		tlsConfig, err := loadTLSCredentials()
		if err != nil {
			return nil, err
		}
		opts = append(opts, xrpc.WithTLSConfig(tlsConfig))
	}

	auth_md := metadata.New(map[string]string{
		"authorization": "bearer " + config.Cfg.AuthToken,
		"kind":          "agent",
		"instance_uuid": vars.InstanceUUID,
		"version":       vars.VersionString,
	})
	ctx = metadata.NewOutgoingContext(ctx, auth_md)

	client := xrpc.NewClient()
	proxyagent_pb.RegisterAgentForProxyServer(client, &ProxyServer{
		proxyConnection: proxyConnection,
	})

	go func() {
		defer cancel()
		backoff := 1 * time.Second
		maxBackoff := 30 * time.Second
		jitterFactor := 0.25
		for {
			jitter := time.Duration((rand.Float64() - 0.5) * jitterFactor * float64(backoff))
			retryInterval := backoff + jitter
			conn, err := client.Dial(ctx, proxyEndpoint, opts...)
			if err == nil {
				select {
				case <-ctx.Done():
					return
				case <-conn.Context().Done():
					if ctx.Err() != nil {
						return
					}
					proxyConnection.rpc.Store(nil)
				}
			} else {
				logger.Err(err).Msg("failed to connect to proxy, retrying in a few seconds...")
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryInterval):
			}
			backoff = min(backoff*2, maxBackoff)
		}
	}()

	return proxyConnection, nil
}

func onLog(logContext *xrpc.LogContext, err error, msg string) {
	logger := zerolog.Ctx(logContext.RpcConnection.Context()).With().Fields(logContext.Fields).Logger()

	if err != nil {
		logger.Err(err).Msg(msg)
	} else {
		logger.Debug().Msg(msg)
	}
}

func onDebugLog(fn xrpc.DebugLogGetter) {
	if config.Cfg.Log.Trace {
		logContext, msg := fn()
		onLog(logContext, nil, fmt.Sprintf("debug: %s", msg))
	}
}
