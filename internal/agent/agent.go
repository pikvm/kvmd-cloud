package agent

import (
	"context"
	"encoding/json"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/pikvm/cloud-api/api_models"
	"github.com/pikvm/cloud-api/domain_errors"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/proxy"
	"github.com/sirupsen/logrus"
)

const (
	retryInterval = 5 * time.Second
	retryJitter   = 2 * time.Second
)

type Agent struct {
	ready atomic.Bool
}

func NewAgent() *Agent {
	return &Agent{
		//
	}
}

// TODO: logs
func (a *Agent) Run(ctx context.Context) error {
	for {
		err := a.Serve(ctx)
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		if err != nil {
			logrus.WithError(err).Error("agent serve error, retrying in a few seconds...")
		}

		jitter := time.Duration(rand.Int64N(int64(retryJitter))) - retryJitter/2
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(retryInterval + jitter):
		}
	}
}

func (a *Agent) Serve(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	proxyEndpoint, err := getProxyEndpoint(ctx)
	if err != nil {
		return err
	}
	conn, err := proxy.Dial(ctx, proxyEndpoint)
	if err != nil {
		return err
	}
	a.ready.Store(true)
	defer a.ready.Store(false)

	select {
	case <-ctx.Done():
		return nil
	case <-conn.Context().Done():
		return nil
	}
}

func getProxyEndpoint(ctx context.Context) (string, error) {
	endpoints, err := getAvailableProxies(ctx)
	if err != nil {
		return "", err
	}
	if len(endpoints) == 0 {
		return "", domain_errors.ErrNoProxiesAvailable
	}

	return selectProxyEndpoint(endpoints), nil
}

func selectProxyEndpoint(endpoints []string) string {
	return endpoints[rand.IntN(len(endpoints))]
}

func getAvailableProxies(ctx context.Context) ([]string, error) {
	httpc := &http.Client{
		Timeout: 5 * time.Second,
	}
	url, err := url.JoinPath(config.Cfg.Hive.Endpoint, "/api/agent/available_proxies")
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := httpc.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var availableProxies api_models.AvailableProxies
	var responseModel api_models.ResponseModel
	responseModel.Result = availableProxies
	if err := json.Unmarshal(respBytes, &responseModel); err != nil {
		return nil, err
	}
	if responseModel.Error != nil {
		return nil, responseModel.Error.ToDomainError()
	}

	endpoints := make([]string, len(availableProxies.Proxies))
	for i, p := range availableProxies.Proxies {
		endpoints[i] = p.Endpoint
	}

	return endpoints, nil
}
