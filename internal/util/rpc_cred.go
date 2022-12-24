package util

import (
	"context"

	"google.golang.org/grpc/credentials"
)

type InsecureRPCCred struct {
	md    map[string]string
	token string
}

func (this InsecureRPCCred) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	result := map[string]string{}
	for k, v := range this.md {
		result[k] = v
	}
	result["authorization"] = "bearer " + this.token
	return result, nil
}

func (this InsecureRPCCred) RequireTransportSecurity() bool {
	return false
}

func NewInsecureRPCCred(md map[string]string, token string) credentials.PerRPCCredentials {
	return InsecureRPCCred{md: md, token: token}
}
