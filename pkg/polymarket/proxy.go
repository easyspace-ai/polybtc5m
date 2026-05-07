package polymarket

import (
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

// proxyURL returns the PROXY_URL env value if set and non-empty.
func proxyURL() *url.URL {
	raw := os.Getenv("PROXY_URL")
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	return u
}

// NewHTTPClientWithProxy creates an http.Client that routes through PROXY_URL
// when the environment variable is set. Exported so that standalone tools (e.g.
// cmd/gen-api-key) can reuse the same proxy configuration as the trader.
func NewHTTPClientWithProxy(timeout time.Duration) *http.Client {
	transport := &http.Transport{}
	if p := proxyURL(); p != nil {
		transport.Proxy = http.ProxyURL(p)
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// newWSDialerWithProxy creates a websocket.Dialer that routes through PROXY_URL
// when the environment variable is set. WebSocket over an HTTP proxy uses the
// CONNECT tunneling method (supported by most modern proxies such as v2ray,
// clash, sing-box, etc.).
func newWSDialerWithProxy() *websocket.Dialer {
	dialer := &websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	if p := proxyURL(); p != nil {
		dialer.Proxy = http.ProxyURL(p)
	}
	return dialer
}
