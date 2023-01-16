package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	hive_pb "github.com/pikvm/cloud-api/hive_for_agent"
	proxy_pb "github.com/pikvm/cloud-api/proxy_for_agent"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/config/vars"
	"github.com/pikvm/kvmd-cloud/internal/util"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type ProxyEventsChannel struct {
	Stream          proxy_pb.ProxyForAgent_EventsChannelClient
	SendEventsQueue chan *EventSendPacket
}

type ProxyConnection struct {
	Ctx         context.Context
	Addr        string
	GrpcConn    *grpc.ClientConn
	ProxyClient proxy_pb.ProxyForAgentClient
	Events      ProxyEventsChannel
}

var (
	proxyConnection *ProxyConnection = nil
)

func loadTLSCredentials() (credentials.TransportCredentials, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	if config.Cfg.SSL.Ca != "" {
		caCert, err := ioutil.ReadFile(config.Cfg.SSL.Ca)
		if err != nil {
			return nil, err
		}
		certPool.AppendCertsFromPEM(caCert)
	}
	config := &tls.Config{
		RootCAs: certPool,
	}
	return credentials.NewTLS(config), nil
}

func Dial(ctx context.Context, proxyInfo *hive_pb.AvailableProxies_ProxyInfo) (*ProxyConnection, error) {
	addr := proxyInfo.GetProxyEndpoint()

	auth_md := map[string]string{
		"agent_uuid": vars.InstanceUUID,
		"agent_name": config.Cfg.AgentName,
	}
	var transportCred grpc.DialOption
	if config.Cfg.NoSSL {
		transportCred = grpc.WithTransportCredentials(insecure.NewCredentials())
	} else {
		tlsCredential, err := loadTLSCredentials()
		if err != nil {
			return nil, err
		}
		transportCred = grpc.WithTransportCredentials(tlsCredential)
	}
	conn, err := grpc.DialContext(
		ctx,
		addr,
		transportCred,
		grpc.WithPerRPCCredentials(util.NewRPCCred(auth_md, config.Cfg.AuthToken)),
	)
	if err != nil {
		return nil, err
	}
	c := proxy_pb.NewProxyForAgentClient(conn)
	log.Debugf("connected to proxy %s", addr)
	proxyConnection = &ProxyConnection{
		Ctx:         ctx,
		Addr:        addr,
		GrpcConn:    conn,
		ProxyClient: c,
		Events: ProxyEventsChannel{
			Stream:          nil,
			SendEventsQueue: make(chan *EventSendPacket),
		},
	}

	return proxyConnection, nil
}
