package proxy

import (
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/pikvm/cloud-api/api_models"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type ProxyPool struct {
	mu          sync.RWMutex
	connections map[string]*ProxyConnection
	updateCh    chan struct{}
}

func NewProxyPool() *ProxyPool {
	return &ProxyPool{
		connections: make(map[string]*ProxyConnection),
		updateCh:    make(chan struct{}),
	}
}

func (p *ProxyPool) Serve(ctx context.Context) {
	logger := log.Logger
	ctx = logger.WithContext(ctx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	updateBaseInterval := 5 * time.Minute
	jitterFactor := 0.25
	jitter := time.Duration((rand.Float64() - 0.5) * jitterFactor * float64(updateBaseInterval))
	updateInterval := updateBaseInterval + jitter

	newEndpointsCh := make(chan []string)

	// endpoint update trigger loop
	go func() {
		defer close(newEndpointsCh)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(updateInterval):
			case <-p.updateCh:
			}
			endpoints := getAvailableProxiesWithRetry(ctx)
			if endpoints == nil {
				continue
			}
			newEndpointsCh <- endpoints
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case endpoints := <-newEndpointsCh:
				logger.Info().Strs("endpoints", endpoints).Msg("received proxy endpoints, updating connections")
				p.updateConnections(ctx, endpoints)
			}
		}
	}()

	p.UpdateEndpoints()
	<-ctx.Done()
}

func (p *ProxyPool) UpdateEndpoints() {
	p.updateCh <- struct{}{}
}

// updateConnections updates the proxy connections based on the new list of endpoints.
// It adds new connections for new endpoints and removes connections for endpoints that are no longer available.
// For not changed endpoints, it keeps the existing connections intact.
func (p *ProxyPool) updateConnections(ctx context.Context, endpoints []string) {
	logger := zerolog.Ctx(ctx)

	p.mu.Lock()
	defer p.mu.Unlock()

	newEndpointsSet := make(map[string]struct{})
	for _, ep := range endpoints {
		newEndpointsSet[ep] = struct{}{}
	}

	// Close all stale connections first
	for ep, conn := range p.connections {
		if _, exists := newEndpointsSet[ep]; !exists {
			conn.Close()
			delete(p.connections, ep)
		}
	}

	// Add new connections
	for _, ep := range endpoints {
		if _, exists := p.connections[ep]; !exists {
			conn, err := ConnectWithRetry(ctx, ep)
			if err != nil {
				logger.Err(err).Str("endpoint", ep).Msg("failed to create proxy connection, skipping")
				continue
			}
			p.connections[ep] = conn
		}
	}
}

func getAvailableProxiesWithRetry(ctx context.Context) []string {
	logger := zerolog.Ctx(ctx)
	backoff := 1 * time.Second
	jitterFactor := 0.25
	for {
		jitter := time.Duration((rand.Float64() - 0.5) * jitterFactor * float64(backoff))
		retryInterval := backoff + jitter
		if proxies, err := getAvailableProxies(ctx); err == nil {
			return proxies
		} else {
			logger.Err(err).Msgf("failed to get available proxies, retrying in %s...", retryInterval)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(retryInterval):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func getAvailableProxies(ctx context.Context) ([]string, error) {
	httpc := &http.Client{
		Timeout: 5 * time.Second,
	}
	url, err := url.JoinPath(config.Cfg.Hive.Endpoint, "/api/agents/get_available_proxies")
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Authorization", "Bearer "+config.Cfg.AuthToken)
	resp, err := httpc.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	availableProxies := api_models.AvailableProxies{}
	response := api_models.ResponseModel{Result: &availableProxies}
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, response.Error.ToDomainError()
	}

	endpoints := make([]string, len(availableProxies.Proxies))
	for i, p := range availableProxies.Proxies {
		endpoints[i] = p.Endpoint
	}

	return endpoints, nil
}
