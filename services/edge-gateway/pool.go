package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/trace"
	"shopkart/pkg/tracing"
)

type Upstream struct {
	RawAddr string
	URL     *url.URL
	Healthy atomic.Bool
	Proxy   *httputil.ReverseProxy
}

type UpstreamInfo struct {
	Address string `json:"address"`
	Healthy bool   `json:"healthy"`
}

type ServicePool struct {
	mu         sync.RWMutex
	name       string
	pathPrefix string
	upstreams  []*Upstream
	rrIndex    uint64
	tracer     trace.Tracer
	client     *http.Client
}

func parseUpstreamAddrs(envVal string, defaultURL string) []string {
	var raw []string
	if envVal != "" {
		for _, part := range strings.Split(envVal, ",") {
			p := strings.TrimSpace(part)
			if p != "" {
				raw = append(raw, p)
			}
		}
	}
	if len(raw) == 0 && defaultURL != "" {
		for _, part := range strings.Split(defaultURL, ",") {
			p := strings.TrimSpace(part)
			if p != "" {
				raw = append(raw, p)
			}
		}
	}
	return raw
}

func NewServicePool(ctx context.Context, name, pathPrefix string, addrs []string, tracer trace.Tracer) *ServicePool {
	pool := &ServicePool{
		name:       name,
		pathPrefix: pathPrefix,
		tracer:     tracer,
		client: &http.Client{
			Timeout: 1500 * time.Millisecond,
		},
	}

	for _, addr := range addrs {
		pool.addUpstreamLocked(addr)
	}

	// Initial health probe
	pool.checkHealth()

	// Periodic health checks every 2 seconds
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pool.checkHealth()
			}
		}
	}()

	return pool
}

func (p *ServicePool) addUpstreamLocked(addr string) *Upstream {
	target := addr
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "http://" + target
	}
	u, err := url.Parse(target)
	if err != nil {
		slog.Error("Invalid upstream URL", slog.String("service", p.name), slog.String("addr", addr), slog.String("error", err.Error()))
		return nil
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       10 * time.Second,
		TLSHandshakeTimeout:   3 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.Transport = transport
	originalDirector := proxy.Director

	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = u.Host
		req.URL.Path = strings.TrimPrefix(req.URL.Path, p.pathPrefix)
		if !strings.HasPrefix(req.URL.Path, "/") {
			req.URL.Path = "/" + req.URL.Path
		}
		tracing.InjectHTTPHeaders(req.Context(), req)
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.ErrorContext(r.Context(), "gateway proxy error",
			slog.String("service", p.name),
			slog.String("target", target),
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
		http.Error(w, fmt.Sprintf(`{"error":"upstream service %s unavailable: %s"}`, p.name, err.Error()), http.StatusBadGateway)
	}

	ups := &Upstream{
		RawAddr: addr,
		URL:     u,
		Proxy:   proxy,
	}
	ups.Healthy.Store(true)
	p.upstreams = append(p.upstreams, ups)
	return ups
}

func (p *ServicePool) AddUpstream(addr string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, u := range p.upstreams {
		if u.RawAddr == addr {
			return
		}
	}
	ups := p.addUpstreamLocked(addr)
	if ups != nil {
		go p.probeUpstream(ups)
	}
}

func (p *ServicePool) RemoveUpstream(addr string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	filtered := make([]*Upstream, 0, len(p.upstreams))
	for _, u := range p.upstreams {
		if u.RawAddr != addr {
			filtered = append(filtered, u)
		}
	}
	p.upstreams = filtered
}

func (p *ServicePool) probeUpstream(u *Upstream) {
	probeURL := fmt.Sprintf("%s://%s/healthz", u.URL.Scheme, u.URL.Host)
	resp, err := p.client.Get(probeURL)
	wasHealthy := u.Healthy.Load()
	if err == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		u.Healthy.Store(true)
		if !wasHealthy {
			slog.Info("Upstream marked HEALTHY", slog.String("service", p.name), slog.String("upstream", u.RawAddr))
		}
	} else {
		if resp != nil {
			_ = resp.Body.Close()
		}
		u.Healthy.Store(false)
		if wasHealthy {
			slog.Warn("Upstream marked UNHEALTHY", slog.String("service", p.name), slog.String("upstream", u.RawAddr))
		}
	}
}

func (p *ServicePool) checkHealth() {
	p.mu.RLock()
	upstreams := make([]*Upstream, len(p.upstreams))
	copy(upstreams, p.upstreams)
	p.mu.RUnlock()

	var wg sync.WaitGroup
	for _, u := range upstreams {
		wg.Add(1)
		go func(ups *Upstream) {
			defer wg.Done()
			p.probeUpstream(ups)
		}(u)
	}
	wg.Wait()
}

func (p *ServicePool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	var healthy []*Upstream
	for _, u := range p.upstreams {
		if u.Healthy.Load() {
			healthy = append(healthy, u)
		}
	}
	allUpstreams := p.upstreams
	p.mu.RUnlock()

	candidates := healthy
	if len(candidates) == 0 {
		candidates = allUpstreams
	}

	if len(candidates) == 0 {
		http.Error(w, fmt.Sprintf(`{"error":"no upstreams configured for %s"}`, p.name), http.StatusServiceUnavailable)
		return
	}

	idx := atomic.AddUint64(&p.rrIndex, 1) - 1
	selected := candidates[idx%uint64(len(candidates))]
	selected.Proxy.ServeHTTP(w, r)
}

func (p *ServicePool) GetStatus() []UpstreamInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	res := make([]UpstreamInfo, len(p.upstreams))
	for i, u := range p.upstreams {
		res[i] = UpstreamInfo{
			Address: u.RawAddr,
			Healthy: u.Healthy.Load(),
		}
	}
	return res
}
