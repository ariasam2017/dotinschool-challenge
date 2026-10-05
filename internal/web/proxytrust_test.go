package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func clientIPOf(t *testing.T, remote string, headers map[string]string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	configureProxyTrust(r)
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

// Through the Cloudflare Tunnel, cloudflared connects from loopback and the
// visitor's real address arrives in CF-Connecting-IP.
func TestClientIPBehindTunnel(t *testing.T) {
	got := clientIPOf(t, "127.0.0.1:51000", map[string]string{"CF-Connecting-IP": "5.160.1.2"})
	if got != "5.160.1.2" {
		t.Fatalf("ClientIP behind tunnel = %q, want the visitor's 5.160.1.2", got)
	}
}

// A direct visitor cannot spoof their IP with forwarding headers.
func TestClientIPNotSpoofableDirectly(t *testing.T) {
	got := clientIPOf(t, "203.0.113.9:40000", map[string]string{
		"CF-Connecting-IP": "1.1.1.1",
		"X-Forwarded-For":  "1.1.1.1",
	})
	if got != "203.0.113.9" {
		t.Fatalf("ClientIP for a direct visitor = %q, want the socket address 203.0.113.9", got)
	}
}
