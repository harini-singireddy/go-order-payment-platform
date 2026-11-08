package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// NewOrderProxy returns a reverse proxy that forwards to the Order service's HTTP port.
func NewOrderProxy(target string) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = u.Scheme
		req.URL.Host = u.Host
		req.Host = u.Host
		// Preserve the original path; the gateway mounts the proxy at /orders
	}
	return proxy, nil
}
