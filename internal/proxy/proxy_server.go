package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"

	agent_pb "github.com/pikvm/cloud-api/proto/agent"
	"github.com/sirupsen/logrus"
	"github.com/xornet-sl/go-xrpc/xrpc"
)

type ProxyServer struct {
	ctx context.Context
	agent_pb.UnimplementedAgentForProxyServer
}

func (this *ProxyServer) ConnectionChannel(stream agent_pb.AgentForProxy_ConnectionChannelServer) error {
	msg, err := stream.Recv()
	if errors.Is(err, xrpc.StreamClosedError) {
		return nil
	}
	if err != nil || msg == nil {
		logrus.WithError(err).Warnf("error while getting data from proxy")
		return fmt.Errorf("error while getting data from proxy: %w", err)
	}
	header := msg.GetHeader()
	if header == nil {
		logrus.Warnf("malformed stream header")
		return fmt.Errorf("malformed stream header")
	}
	cid := header.GetCid()
	connectTo := header.GetConnectTo()

	var conn net.Conn
	if connectTo[0] == '/' {
		conn, err = net.Dial("unix", connectTo)
	} else {
		conn, err = net.Dial("tcp", connectTo)
	}
	if err != nil {
		return err
	}

	if err := stream.Send(&agent_pb.ConnectionMessage{
		Body: &agent_pb.ConnectionMessage_HeaderResponse_{
			HeaderResponse: &agent_pb.ConnectionMessage_HeaderResponse{
				Error: "",
			},
		},
	}); err != nil {
		return nil
	}

	logrus.Debugf("Connection cid:%s created", cid)
	defer logrus.Debugf("Connection cid:%s closed", cid)
	defer conn.Close()

	senderError := make(chan error)
	receiverError := make(chan error)

	// Proxy -> Inner
	go func() {
		defer close(senderError)
		for {
			msg, err := stream.Recv()
			if errors.Is(err, io.EOF) || errors.Is(err, xrpc.StreamClosedError) {
				// Connection closed on proxy's side
				conn.Close()
				return
			}
			if err != nil {
				logrus.WithError(err).Errorf("error while getting data from proxy")
				conn.Close()
				senderError <- err
				return
			}
			chunk := msg.GetChunk()
			if _, err := conn.Write(chunk); err != nil {
				logrus.WithError(err).Errorf("unable to send data to inner connection")
				conn.Close()
				// apiHandler.Notify(apiHandler.Ctx, "connectionClosed", connection.Cid)
				senderError <- err
				return
			}
		}
	}()
	// Inner -> Proxy
	go func() {
		defer close(receiverError)
		readCloserChan := make(chan struct{})
		go func() {
			select {
			case <-this.ctx.Done():
				conn.Close()
				return
			case <-readCloserChan:
				return
			}
		}()
		defer close(readCloserChan)
		buff := make([]byte, 2048)
		for {
			n, err := conn.Read(buff)
			if isNetConnClosedErr(err) {
				return
			} else if err != nil {
				logrus.WithError(err).Errorf("error reading from inner connection")
				conn.Close()
				receiverError <- err
				return
			}
			err = stream.Send(&agent_pb.ConnectionMessage{
				Body: &agent_pb.ConnectionMessage_Chunk{
					Chunk: buff[:n],
				},
			})
			if errors.Is(err, xrpc.StreamClosedError) {
				conn.Close()
				return
			}
			if err != nil {
				logrus.WithError(err).Errorf("unable to send data to proxy")
				conn.Close()
				receiverError <- err
				return
			}
		}
	}()

	select {
	case <-this.ctx.Done():
		return nil
	case err := <-senderError:
		return err
	case err := <-receiverError:
		return err
	}
}

func isNetConnClosedErr(err error) bool {
	switch {
	case
		errors.Is(err, net.ErrClosed),
		errors.Is(err, io.EOF),
		errors.Is(err, syscall.EPIPE):
		return true
	default:
		return false
	}
}
