package server

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"picture-this/internal/config"
)

func newTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	resetTestAuthTokens()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping test; listen unavailable: %v", err)
	}
	ts := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: handler},
	}
	ts.Start()
	return ts
}

func newServerHarness(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	return newServerHarnessWithConfig(t, config.Default())
}

func newServerHarnessWithConfig(t *testing.T, cfg config.Config) (*Server, *httptest.Server) {
	t.Helper()
	srv := New(nil, cfg)
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)
	return srv, ts
}
