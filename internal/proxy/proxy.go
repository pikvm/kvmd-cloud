package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	proxyagent_pb "github.com/pikvm/cloud-api/proto/proxyagent"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/config/vars"
	"github.com/sirupsen/logrus"
	"github.com/xornet-sl/go-xrpc/xrpc"
	"google.golang.org/grpc/metadata"
)

type ProxyConnection struct {
	ctx  context.Context
	Addr string
	Rpc  *xrpc.RpcConn
}

var (
	proxyConnection *ProxyConnection = nil
)

func (this *ProxyConnection) Context() context.Context {
	return this.ctx
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

func Dial(ctx context.Context, proxyEndpoint string) (*ProxyConnection, error) {
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
		ctx: ctx,
	})
	conn, err := client.Dial(ctx, proxyEndpoint, opts...)
	if err != nil {
		return nil, err
	}

	proxyConnection = &ProxyConnection{
		ctx:  conn.Context(),
		Addr: proxyEndpoint,
		Rpc:  conn,
	}

	return proxyConnection, nil
}

func onOpen(ctx context.Context, conn *xrpc.RpcConn) (context.Context, error) {
	logrus.Debugf("connected to proxy %s", conn.RemoteAddr().String())
	return nil, nil
}

func onClosed(ctx context.Context, conn *xrpc.RpcConn, closeError error) {
	logrus.WithError(closeError).Debugf("connection to proxy %s lost", conn.RemoteAddr().String())
}

func onLog(logContext *xrpc.LogContext, err error, msg string) {
	if err != nil {
		logrus.WithFields(logContext.Fields).WithError(err).Error(msg)
	} else {
		logrus.WithFields(logContext.Fields).Debug(msg)
	}
}

func onDebugLog(fn xrpc.DebugLogGetter) {
	if config.Cfg.Log.Trace {
		logContext, msg := fn()
		onLog(logContext, nil, fmt.Sprintf("debug: %s", msg))
	}
}
