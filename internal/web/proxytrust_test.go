package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func clientIPOf(t *testing.T, remote string, headers map[string]string, header string, trusted ...string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	configureProxyTrust(r, trusted, header)
	var got string
	r.GET("/ip", func(c *gin.Context) { got = c.ClientIP() })
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = remote
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	r.ServeHTTP(httptest.NewRecorder(), req)
	return got
}

// Through the Cloudflare Tunnel (CLIENT_IP_HEADER=CF-Connecting-IP),
// cloudflared connects from loopback and the visitor's real address arrives in
// CF-Connecting-IP.
func TestClientIPBehindTunnel(t *testing.T) {
	got := clientIPOf(t, "127.0.0.1:51000", map[string]string{"CF-Connecting-IP": "5.160.1.2"}, "CF-Connecting-IP")
	if got != "5.160.1.2" {
		t.Fatalf("ClientIP behind tunnel = %q, want the visitor's 5.160.1.2", got)
	}
}

// Only the configured header counts: with the default X-Forwarded-For, a
// CF-Connecting-IP (or X-Real-IP) passed through a proxy is ignored, so a
// second header can't be used to pick a fresh rate-limit bucket.
func TestClientIPSingleHeader(t *testing.T) {
	got := clientIPOf(t, "127.0.0.1:51000", map[string]string{
		"X-Forwarded-For":  "5.160.1.2",
		"CF-Connecting-IP": "1.1.1.1",
		"X-Real-IP":        "2.2.2.2",
	}, "")
	if got != "5.160.1.2" {
		t.Fatalf("ClientIP = %q, want X-Forwarded-For's 5.160.1.2", got)
	}
}

// A direct visitor cannot spoof their IP with forwarding headers.
func TestClientIPNotSpoofableDirectly(t *testing.T) {
	got := clientIPOf(t, "203.0.113.9:40000", map[string]string{
		"CF-Connecting-IP": "1.1.1.1",
		"X-Forwarded-For":  "1.1.1.1",
	}, "")
	if got != "203.0.113.9" {
		t.Fatalf("ClientIP for a direct visitor = %q, want the socket address 203.0.113.9", got)
	}
}

// Behind a Caddy container on the Docker network (TRUSTED_PROXIES), the
// visitor comes from X-Forwarded-For — and a junk entry in the list is ignored.
func TestClientIPBehindDockerCaddy(t *testing.T) {
	got := clientIPOf(t, "172.18.0.3:41000", map[string]string{"X-Forwarded-For": "5.160.1.2"}, "", "172.16.0.0/12", "not-an-ip")
	if got != "5.160.1.2" {
		t.Fatalf("ClientIP behind Docker Caddy = %q, want the visitor's 5.160.1.2", got)
	}
	// still not spoofable from outside that network
	got = clientIPOf(t, "203.0.113.9:40000", map[string]string{"X-Forwarded-For": "1.1.1.1"}, "", "172.16.0.0/12")
	if got != "203.0.113.9" {
		t.Fatalf("ClientIP for a direct visitor = %q, want 203.0.113.9", got)
	}
}
