package main

import (
	"bufio"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestAPIHandlerServesNeutralPlaceholder(t *testing.T) {
	c := config{APIKey: strings.Repeat("k", 32), Upstream: "https://api.telegram.org"}
	h, err := apiHandler(c, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/", "/favicon.ico", "/admin", "/anything"} {
		response := httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Errorf("%s: got status %d", path, response.Code)
		}
		if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
			t.Errorf("%s: unexpected content type %q", path, contentType)
		}
		body := strings.ToLower(response.Body.String())
		for _, sensitive := range []string{"telegram", "proxy", "tgproxy"} {
			if strings.Contains(body, sensitive) {
				t.Errorf("%s: placeholder exposes %q", path, sensitive)
			}
		}
		for _, expected := range []string{"Яна", "Анатолий", "18 декабря 2027", "Avemori"} {
			if !strings.Contains(response.Body.String(), expected) {
				t.Errorf("%s: placeholder is missing %q", path, expected)
			}
		}
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

func TestIsDomain(t *testing.T) {
	for _, host := range []string{"proxy.example.com", "tg-api.example.ru", "example.com."} {
		if !isDomain(host) {
			t.Errorf("expected domain: %s", host)
		}
	}
	for _, host := range []string{"", "127.0.0.1", "2001:db8::1", "localhost", "-bad.example.com", "bad_.example.com"} {
		if isDomain(host) {
			t.Errorf("expected invalid domain: %s", host)
		}
	}
}

func TestAnswerYes(t *testing.T) {
	for _, answer := range []string{"да", "Д", "yes", "Y", "1"} {
		if !answerYes(answer) {
			t.Errorf("expected yes: %s", answer)
		}
	}
	if answerYes("нет") {
		t.Error("expected no")
	}
}

func TestValidPort(t *testing.T) {
	for _, port := range []string{"1", "443", "65535"} {
		if !validPort(port) {
			t.Errorf("expected valid port: %s", port)
		}
	}
	for _, port := range []string{"", "0", "65536", "https", "443\nBAD=value"} {
		if validPort(port) {
			t.Errorf("expected invalid port: %q", port)
		}
	}
}

func TestWizardRetriesInvalidPortAndWritesHTTPSConfig(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	// Empty HTTPS answer selects the default "yes"; the accidental word
	// entered as a port must be rejected and requested again.
	input := bufio.NewReader(strings.NewReader("u-nas-budet-svadba.ru\n\nда\n8443\n"))
	if err := wizard(input); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(".env")
	if err != nil {
		t.Fatal(err)
	}
	config := string(content)
	for _, expected := range []string{
		"TGPROXY_PUBLIC_HOST=u-nas-budet-svadba.ru",
		"TGPROXY_MTG_PORT=8443",
		"TGPROXY_API_PORT=443",
		"COMPOSE_PROFILES=https",
	} {
		if !strings.Contains(config, expected) {
			t.Errorf("missing %q in generated config", expected)
		}
	}
}
