package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAPIHandlerAuthorizationAndRedaction(t *testing.T) {
	c := config{APIKey: strings.Repeat("k", 32), Upstream: "https://api.telegram.org"}
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.RawQuery, "proxy_key") {
			t.Error("proxy key leaked upstream")
		}
		return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: http.NoBody, Request: r}, nil
	})
	h, err := apiHandler(c, slog.New(slog.NewTextHandler(io.Discard, nil)), transport)
	if err != nil {
		t.Fatal(err)
	}

	unauthorized := httptest.NewRecorder()
	h.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/bot123:secret/getMe", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", unauthorized.Code)
	}

	authorized := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bot123:secret/getMe?proxy_key="+c.APIKey, nil)
	h.ServeHTTP(authorized, req)
	if authorized.Code != http.StatusNoContent {
		t.Fatalf("got %d", authorized.Code)
	}
	if got := redactedPath(req.URL.Path); strings.Contains(got, "secret") {
		t.Fatalf("token was not redacted: %s", got)
	}
}

func TestValidTelegramPath(t *testing.T) {
	for _, path := range []string{"/bot123:getMe/getMe", "/file/bot123:file/documents/a"} {
		if !validTelegramPath(path) {
			t.Errorf("expected valid: %s", path)
		}
	}
	for _, path := range []string{"/", "/healthz", "/proxy/http://example.com"} {
		if validTelegramPath(path) {
			t.Errorf("expected invalid: %s", path)
		}
	}
}
