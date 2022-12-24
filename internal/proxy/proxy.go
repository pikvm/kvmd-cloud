package proxy

import (
	"context"

	hive_pb "github.com/pikvm/cloud-api/hive_for_agent"
	proxy_pb "github.com/pikvm/cloud-api/proxy_for_agent"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/config/vars"
	"github.com/pikvm/kvmd-cloud/internal/util"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
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

func Dial(ctx context.Context, proxyInfo *hive_pb.AvailableProxies_ProxyInfo) error {
	addr := proxyInfo.GetProxyEndpoint()

	auth_md := map[string]string{
		"agent_uuid": vars.InstanceUUID,
		"agent_name": config.Cfg.AgentName,
	}
	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(util.NewInsecureRPCCred(auth_md, config.Cfg.AuthToken)),
	)
	if err != nil {
		return err
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

	if err = processEvents(proxyConnection); err != nil {
		return err
	}

	log.Debugf("connection to proxy %s lost", addr)

	return nil
}
