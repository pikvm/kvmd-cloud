package proxy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"

	proxyagent_pb "github.com/pikvm/cloud-api/proto/proxyagent"
	"github.com/rs/zerolog"
	"github.com/xornet-sl/go-xrpc/xrpc"
)

type ProxyServer struct {
	proxyConnection *ProxyConnection
	proxyagent_pb.UnimplementedAgentForProxyServer
}

func (this *ProxyServer) ConnectionChannel(stream proxyagent_pb.AgentForProxy_ConnectionChannelServer) error {
	rpcConn := this.proxyConnection.GetRpcConn()
	if rpcConn == nil {
		return fmt.Errorf("rpc connection is nil")
	}
	logger := zerolog.Ctx(rpcConn.Context())

	msg, err := stream.Recv()
	if errors.Is(err, xrpc.StreamClosedError) {
		return nil
	}
	if err != nil || msg == nil {
		logger.Warn().Err(err).Msg("error while getting data from proxy")
		return fmt.Errorf("error while getting data from proxy: %w", err)
	}
	header := msg.GetHeader()
	if header == nil {
		logger.Warn().Msg("malformed stream header")
		return fmt.Errorf("malformed stream header")
	}
	cid := header.GetCid()
	connectTo := header.GetConnectTo()
	cidLogger := logger.With().Str("cid", cid).Logger()

	var conn net.Conn
	if connectTo[0] == '/' {
		conn, err = net.Dial("unix", connectTo)
	} else {
		conn, err = net.Dial("tcp", connectTo)
	}
	if err != nil {
		return err
	}

	if err := stream.Send(&proxyagent_pb.ConnectionMessage{
		Body: &proxyagent_pb.ConnectionMessage_HeaderResponse_{
			HeaderResponse: &proxyagent_pb.ConnectionMessage_HeaderResponse{
				Error: "",
			},
		},
	}); err != nil {
		return nil
	}

	cidLogger.Debug().Msg("Connection created")
	defer cidLogger.Debug().Msg("Connection closed")
	defer conn.Close()

	senderError := make(chan error)
	receiverError := make(chan error)

	// Proxy -> Inner socket
	go func() {
		defer close(senderError)
		for {
			msg, err := stream.Recv()
			if isNetConnClosedErr(err) || errors.Is(err, xrpc.StreamClosedError) {
				// Connection closed on proxy's side
				cidLogger.Trace().Msg("proxy->inner rpc read closed")
				conn.Close()
				return
			}
			if err != nil {
				cidLogger.Err(err).Msg("error while getting data from proxy")
				conn.Close()
				senderError <- err
				return
			}
			chunk := msg.GetChunk()
			cidLogger.Trace().Msgf("proxy->inner rpc received %d bytes", len(chunk))
			n, err := conn.Write(chunk)
			cidLogger.Trace().Msgf("inner written %d bytes", n)
			if err != nil {
				cidLogger.Err(err).Msg("unable to send data to inner connection")
				conn.Close()
				// apiHandler.Notify(apiHandler.Ctx, "connectionClosed", connection.Cid)
				senderError <- err
				return
			}
		}
	}()
	// Inner socket -> Proxy
	go func() {
		defer close(receiverError)
		readCloserChan := make(chan struct{})
		go func() {
			connCtx := this.proxyConnection.GetRpcConn().Context()
			if connCtx == nil {
				return
			}
			select {
			case <-connCtx.Done():
				conn.Close()
				return
			case <-readCloserChan:
				return
			}
		}()
		defer close(readCloserChan)
		buff := make([]byte, 8192)
		for {
			n, err := conn.Read(buff)
			cidLogger.Trace().Msgf("inner read: %d bytes", n)
			if isNetConnClosedErr(err) {
				return
			} else if err != nil {
				cidLogger.Err(err).Msg("error reading from inner connection")
				conn.Close()
				receiverError <- err
				return
			}
			err = stream.Send(&proxyagent_pb.ConnectionMessage{
				Body: &proxyagent_pb.ConnectionMessage_Chunk{
					Chunk: buff[:n],
				},
			})
			if errors.Is(err, xrpc.StreamClosedError) {
				cidLogger.Trace().Msg("inner->proxy rpc send closed")
				conn.Close()
				return
			}
			if err != nil {
				cidLogger.Err(err).Msg("unable to send data to proxy")
				conn.Close()
				receiverError <- err
				return
			}
			cidLogger.Trace().Msgf("inner->proxy rpc sent %d bytes", n)
		}
	}()

	proxyConn := this.proxyConnection.GetRpcConn().Context()
	if proxyConn == nil {
		return nil
	}
	select {
	case <-proxyConn.Done():
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
