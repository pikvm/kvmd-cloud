package hive

import (
	"fmt"
	"io"
	"time"

	hive_pb "github.com/pikvm/cloud-api/hive_for_agent"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/config/vars"
	log "github.com/sirupsen/logrus"
)

const (
	pingInterval = 2 * time.Second
)

type HiveEventsChannel struct {
	Stream          hive_pb.HiveForAgent_EventsChannelClient
	SendEventsQueue chan *EventSendPacket
}

type EventSendPacket struct {
	Event *hive_pb.AgentEvent
	Error chan error
}

func (this *HiveEventsChannel) Send(event *hive_pb.AgentEvent) error {
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

func ProcessEvents(hconn *HiveConnection) error {
	stream, err := hconn.HiveClient.EventsChannel(hconn.Ctx)
	if err != nil {
		return err
	}

	var evId uint32 = 0

	NextEventId := func() uint32 {
		evId += 1
		return evId
	}

	// Register agent in hive's side
	err = stream.Send(&hive_pb.AgentEvent{
		Id:         NextEventId(),
		IsResponse: false,
		Type:       hive_pb.AgentEventType_AETYPE_REGISTER,
		Body: &hive_pb.AgentEvent_AgentInfo_{
			AgentInfo: &hive_pb.AgentEvent_AgentInfo{
				Name:    config.Cfg.AgentName,
				Version: vars.Version,
			},
		},
	})
	if err != nil {
		log.WithError(err).Errorf("unable to register on hive %s", hconn.Addr)
		stream.CloseSend()
		return err
	}
	// TODO: timeouts
	pingsWhileRegister := 0
	var event *hive_pb.HiveEvent
	for {
		if pingsWhileRegister >= 3 {
			err := fmt.Errorf("unable to register on give %s: timeout", hconn.Addr)
			log.Errorf(err.Error())
			stream.CloseSend()
			return err
		}
		pingsWhileRegister += 1
		event, err = stream.Recv()
		if err != nil {
			log.WithError(err).Errorf("unable to register on hive %s", hconn.Addr)
			return err
		}
		if event.GetType() == hive_pb.HiveEventType_HETYPE_PING {
			continue
		}
		if event.GetType() != hive_pb.HiveEventType_HETYPE_OK ||
			event.GetId() != evId ||
			event.GetIsResponse() != true {
			err = fmt.Errorf("unable to register on hive %s: malformed response %#+v", hconn.Addr, event)
			log.Errorf(err.Error())
			stream.CloseSend()
			return err
		}
		break
	}
	log.Debugf("registered on hive %s", hconn.Addr)

	hconn.Events.Stream = stream

	// Sender
	go func() {
		ticker := time.NewTicker(pingInterval)
		for {
			select {
			case <-hconn.Ctx.Done():
				return
			case <-ticker.C:
				if err := stream.Send(&hive_pb.AgentEvent{
					Id:         0, //pings are always 0
					Type:       hive_pb.AgentEventType_AETYPE_PING,
					IsResponse: false,
					Body: &hive_pb.AgentEvent_Ping{
						Ping: nil,
					},
				}); err != nil {
					stream.CloseSend()
					hconn.GrpcConn.Close()
					return
				}
				log.Tracef("PING sent: Hive")
			case event, ok := <-hconn.Events.SendEventsQueue:
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
			// hive lost
			return nil
		}
		if err != nil {
			return err
		}
		switch event.GetType() {
		case hive_pb.HiveEventType_HETYPE_PING:
			// TODO: reset ping watchdog timer
			log.Tracef("PING recv: Hive")
		default:
			// Unknown event
			log.Debugf("unknown event type %s received from hive %s", event.GetType().String(), hconn.Addr)
		}
	}
}
