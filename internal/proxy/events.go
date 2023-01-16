package proxy

import (
	"fmt"
	"io"
	"time"

	pb "github.com/pikvm/cloud-api/proxy_for_agent"
	"github.com/pikvm/kvmd-cloud/internal/config"
	log "github.com/sirupsen/logrus"
)

const (
	pingInterval = 2 * time.Second
)

type EventSendPacket struct {
	Event *pb.AgentEvent
	Error chan error
}

func (this *ProxyEventsChannel) Send(event *pb.AgentEvent) error {
	if this.Stream == nil {
		return fmt.Errorf("events stream is not open")
	}
	packet := &EventSendPacket{
		Event: event,
		Error: make(chan error),
	}
	defer close(packet.Error)
	this.SendEventsQueue <- packet
	return <-packet.Error
}

func ProcessEvents(pconn *ProxyConnection) error {
	stream, err := pconn.ProxyClient.EventsChannel(pconn.Ctx)
	if err != nil {
		return err
	}

	var evId uint32 = 0

	NextEventId := func() uint32 {
		evId += 1
		return evId
	}

	// Register agent on proxy's side
	err = stream.Send(&pb.AgentEvent{
		Id:         NextEventId(),
		IsResponse: false,
		Type:       pb.AgentEventType_AETYPE_REGISTER,
		Body: &pb.AgentEvent_AgentInfo_{
			AgentInfo: &pb.AgentEvent_AgentInfo{
				Name: config.Cfg.AgentName,
			},
		},
	})
	if err != nil {
		log.WithError(err).Errorf("unable to register on proxy %s", pconn.Addr)
		stream.CloseSend()
		return err
	}
	defer log.Debugf("connection to proxy %s lost", pconn.Addr)
	// TODO: timeouts
	pingsWhileRegister := 0
	var event *pb.ProxyEvent
	for {
		if pingsWhileRegister >= 3 {
			err := fmt.Errorf("unable to register on proxy %s: timeout", pconn.Addr)
			log.Errorf(err.Error())
			stream.CloseSend()
			return err
		}
		pingsWhileRegister += 1
		event, err = stream.Recv()
		if err != nil {
			log.WithError(err).Errorf("unable to register on proxy %s", pconn.Addr)
			return err
		}
		if event.GetType() == pb.ProxyEventType_PETYPE_PING {
			continue
		}
		if event.GetType() != pb.ProxyEventType_PETYPE_OK ||
			event.GetId() != evId ||
			event.GetIsResponse() != true {
			err = fmt.Errorf("unable to register on proxy %s: malformed response: %#+v", pconn.Addr, event)
			log.Errorf(err.Error())
			stream.CloseSend()
			return err
		}
		break
	}
	log.Debugf("registered on proxy %s", pconn.Addr)

	pconn.Events.Stream = stream

	// Sender
	go func() {
		ticker := time.NewTicker(pingInterval)
		for {
			select {
			case <-pconn.Ctx.Done():
				return
			case <-ticker.C:
				if err := stream.Send(&pb.AgentEvent{
					Id:         0, // pings are always 0
					Type:       pb.AgentEventType_AETYPE_PING,
					IsResponse: false,
					Body: &pb.AgentEvent_Ping{
						Ping: nil,
					},
				}); err != nil {
					stream.CloseSend()
					pconn.GrpcConn.Close()
					return
				}
			case event, ok := <-pconn.Events.SendEventsQueue:
				if !ok {
					// Someone closed this channel for a reason. Quit silently
					stream.CloseSend()
					return
				}
				event.Error <- stream.Send(event.Event)
			}
		}
	}()
	// Receiver
	for {
		event, err := stream.Recv()
		if err == io.EOF {
			// proxy lost
			return nil
		}
		if err != nil {
			return err
		}
		switch event.GetType() {
		case pb.ProxyEventType_PETYPE_PING:
			// TODO: reset ping watchdog timer
		case pb.ProxyEventType_PETYPE_NEW_CONNECTION:
			// Process a new connection
			if err = startNewConnection(pconn, event.GetNewConnection()); err != nil {
				if err = pconn.Events.Send(&pb.AgentEvent{
					Id:         event.GetId(),
					Type:       pb.AgentEventType_AETYPE_ERROR,
					IsResponse: true,
					Body: &pb.AgentEvent_Error{
						Error: err.Error(),
					},
				}); err != nil {
					return err
				}
			} else if err = pconn.Events.Send(&pb.AgentEvent{
				Id:         event.GetId(),
				Type:       pb.AgentEventType_AETYPE_OK,
				IsResponse: true,
			}); err != nil {
				return err
			}
		default:
			// Unknown event
			log.Debugf("Unknown event type %s received from proxy %s", event.GetType().String(), pconn.Addr)
		}
	}
}
