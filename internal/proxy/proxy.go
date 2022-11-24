package proxy

import (
	"context"

	"github.com/google/uuid"
	pb "github.com/pikvm/cloud-api/proxy"
	"github.com/pikvm/kvmd-cloud/internal/config"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type ProxyEventsChannel struct {
	Stream          pb.Proxy_EventsChannelClient
	SendEventsQueue chan *EventSendPacket
}

type ProxyConnection struct {
	Ctx         context.Context
	Addr        string
	GrpcConn    *grpc.ClientConn
	ProxyClient pb.ProxyClient
	Events      ProxyEventsChannel
}

var (
	proxyConnection *ProxyConnection = nil
)

func Dial(ctx context.Context) error {
	addr := config.Cfg.ProxyAddress

	uuid := uuid.New()
	md := metadata.Pairs("agent_uuid", uuid.String())
	dialCtx := metadata.NewOutgoingContext(ctx, md)
	conn, err := grpc.DialContext(dialCtx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	c := pb.NewProxyClient(conn)
	log.Debugf("connected to proxy %s", addr)
	proxyConnection = &ProxyConnection{
		Addr:        addr,
		Ctx:         dialCtx,
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
