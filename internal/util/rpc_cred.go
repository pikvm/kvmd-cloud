package util

import (
	"context"

	"google.golang.org/grpc/credentials"
)

type RPCCred struct {
	md    map[string]string
	token string
}

func (this RPCCred) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	result := map[string]string{}
	for k, v := range this.md {
		result[k] = v
	}
	result["authorization"] = "bearer " + this.token
	return result, nil
}

func (this RPCCred) RequireTransportSecurity() bool {
	return false
}

func NewRPCCred(md map[string]string, token string) credentials.PerRPCCredentials {
	return RPCCred{md: md, token: token}
}
