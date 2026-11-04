package web

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/dotinschool/dotinschool-challenge/internal/config"
	"github.com/dotinschool/dotinschool-challenge/internal/database"
	"github.com/dotinschool/dotinschool-challenge/internal/service"
)

const caddyIP = "172.30.57.10"

type transportRig struct {
	router *gin.Engine
	admin  string
}

func newTransportRig(t *testing.T, cfg *config.Config) *transportRig {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	log := logrus.New()
	log.SetOutput(io.Discard)
	cfg.AdminUsername = "administrator"
	cfg.AdminPath = "/secret-admin"
	cfg.DataDir = t.TempDir()
	cfg.TrustedProxies = []string{caddyIP}
	svc := service.New(cfg, db, log)
	return &transportRig{
		router: NewRouter(Options{Config: cfg, Services: svc, Log: log, Version: "test"}),
		admin:  svc.Sessions.IssueAdmin("administrator"),
	}
}

// req sends a request from remote (ip:port) with the given headers.
func (r *transportRig) req(method, path, remote string, headers map[string]string) *httptest.ResponseRecorder {
	var body io.Reader
	if method != http.MethodGet && method != http.MethodHead {
		body = strings.NewReader("{}")
	}
	req := httptest.NewRequest(method, path, body)
	req.RemoteAddr = remote
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.router.ServeHTTP(rec, req)
	return rec
}

var viaCaddyHTTPS = map[string]string{"X-Forwarded-For": "5.160.1.2", "X-Forwarded-Proto": "https"}

// Production serves nothing over plain HTTP — not the API, not the SPA — and a
// direct visitor can't claim https with a forged X-Forwarded-Proto.
func TestProductionRefusesPlainHTTP(t *testing.T) {
	r := newTransportRig(t, &config.Config{Production: true, TLSProxy: true, AdminAllowAll: true})

	for _, path := range []string{"/api/config", "/"} {
		if rec := r.req(http.MethodGet, path, "203.0.113.9:40000", nil); rec.Code != http.StatusForbidden {
			t.Errorf("plain HTTP %s = %d, want 403", path, rec.Code)
		}
		forged := map[string]string{"X-Forwarded-Proto": "https"}
		if rec := r.req(http.MethodGet, path, "203.0.113.9:40000", forged); rec.Code != http.StatusForbidden {
			t.Errorf("forged X-Forwarded-Proto on %s = %d, want 403", path, rec.Code)
		}
	}
	if rec := r.req(http.MethodGet, "/api/config", "203.0.113.9:40000", nil); codeOf(rec) != "TLS_REQUIRED" {
		t.Errorf("code = %q, want TLS_REQUIRED", codeOf(rec))
	}
	// the proxy itself forwarding a plain-HTTP visitor is refused too
	plainViaCaddy := map[string]string{"X-Forwarded-For": "5.160.1.2", "X-Forwarded-Proto": "http"}
	if rec := r.req(http.MethodGet, "/api/config", caddyIP+":4000", plainViaCaddy); rec.Code != http.StatusForbidden {
		t.Errorf("plain HTTP through Caddy = %d, want 403", rec.Code)
	}
}

// Through the TLS proxy everything works and HSTS is sent; the container's
// own loopback healthcheck still works, without HSTS.
func TestProductionThroughTLSProxy(t *testing.T) {
	r := newTransportRig(t, &config.Config{Production: true, TLSProxy: true, AdminAllowAll: true})

	rec := r.req(http.MethodGet, "/api/config", caddyIP+":4000", viaCaddyHTTPS)
	if rec.Code != http.StatusOK {
		t.Fatalf("via Caddy https = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Strict-Transport-Security"); got != hstsValue {
		t.Errorf("HSTS = %q, want %q", got, hstsValue)
	}

	rec = r.req(http.MethodGet, "/api/config", "127.0.0.1:5000", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("loopback healthcheck = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Errorf("HSTS sent on a plain loopback request")
	}
}

// The admin surface needs TLS even outside production.
func TestAdminRequiresTLSInEveryMode(t *testing.T) {
	r := newTransportRig(t, &config.Config{})

	if rec := r.req(http.MethodPost, "/api/admin/login", "203.0.113.9:40000", nil); codeOf(rec) != "TLS_REQUIRED" {
		t.Errorf("plain-HTTP admin login: code %q (%d), want TLS_REQUIRED", codeOf(rec), rec.Code)
	}
	h := map[string]string{"Authorization": "Bearer " + r.admin}
	if rec := r.req(http.MethodGet, "/api/participants", "203.0.113.9:40000", h); codeOf(rec) != "TLS_REQUIRED" {
		t.Errorf("plain-HTTP admin API: code %q (%d), want TLS_REQUIRED", codeOf(rec), rec.Code)
	}

	// over TLS (via the proxy) the channel is fine: login reaches the
	// credential check, the token works
	if rec := r.req(http.MethodPost, "/api/admin/login", caddyIP+":4000", viaCaddyHTTPS); rec.Code == http.StatusForbidden {
		t.Errorf("admin login over TLS refused: %s", rec.Body.String())
	}
	h["X-Forwarded-For"], h["X-Forwarded-Proto"] = "5.160.1.2", "https"
	if rec := r.req(http.MethodGet, "/api/participants", caddyIP+":4000", h); rec.Code != http.StatusOK {
		t.Errorf("admin API over TLS = %d, want 200", rec.Code)
	}
	// local plain HTTP (dev run, SSH tunnel) is accepted
	if rec := r.req(http.MethodGet, "/api/participants", "127.0.0.1:5000", map[string]string{"Authorization": "Bearer " + r.admin}); rec.Code != http.StatusOK {
		t.Errorf("local admin API = %d, want 200", rec.Code)
	}
}

// Outside ADMIN_ALLOWED_CIDRS the admin panel, login and API don't exist —
// even with a valid token — while the user panel is unaffected.
func TestAdminNetworkAllowList(t *testing.T) {
	_, mgmt, _ := net.ParseCIDR("10.20.0.0/16")
	r := newTransportRig(t, &config.Config{AdminAllowedNets: []*net.IPNet{mgmt}})

	outside := map[string]string{"X-Forwarded-For": "5.160.1.2", "X-Forwarded-Proto": "https", "Authorization": "Bearer " + r.admin}
	inside := map[string]string{"X-Forwarded-For": "10.20.3.4", "X-Forwarded-Proto": "https", "Authorization": "Bearer " + r.admin}

	cases := []struct{ method, path string }{
		{http.MethodGet, "/secret-admin/"},
		{http.MethodPost, "/api/admin/login"},
		{http.MethodGet, "/api/participants"},
		{http.MethodGet, "/api/admin/backup"},
	}
	for _, tc := range cases {
		if rec := r.req(tc.method, tc.path, caddyIP+":4000", outside); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s from outside = %d, want 404", tc.method, tc.path, rec.Code)
		}
	}
	if rec := r.req(http.MethodGet, "/secret-admin/", caddyIP+":4000", inside); rec.Code != http.StatusOK {
		t.Errorf("admin panel from the management network = %d, want 200", rec.Code)
	}
	if rec := r.req(http.MethodGet, "/api/participants", caddyIP+":4000", inside); rec.Code != http.StatusOK {
		t.Errorf("admin API from the management network = %d, want 200", rec.Code)
	}
	if rec := r.req(http.MethodGet, "/", caddyIP+":4000", outside); rec.Code != http.StatusOK {
		t.Errorf("user panel from outside = %d, want 200", rec.Code)
	}
}

// publicRoutes are the only API routes reachable without an admin token. Any
// route added to mountAPI that is not listed here must sit behind
// requireAdmin — this test fails otherwise.
var publicRoutes = map[string]bool{
	"POST /api/otp/request":       true,
	"POST /api/register":          true,
	"POST /api/lookup":            true,
	"POST /api/session":           true,
	"GET /api/config":             true,
	"GET /api/version":            true,
	"POST /api/client-errors":     true,
	"PATCH /api/participants/:id": true, // participant session OR admin token, checked in the handler
	"POST /api/admin/login":       true,
}

func TestEveryNonPublicRouteRequiresAdmin(t *testing.T) {
	r := newTransportRig(t, &config.Config{AllowInsecureAdmin: true})
	checked := 0
	for _, rt := range r.router.Routes() {
		if !strings.HasPrefix(rt.Path, "/api/") || publicRoutes[rt.Method+" "+rt.Path] {
			continue
		}
		path := strings.ReplaceAll(rt.Path, ":id", "x")
		rec := r.req(rt.Method, path, "127.0.0.1:5000", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without a token = %d, want 401 — admin routes must use requireAdmin", rt.Method, rt.Path, rec.Code)
		}
		checked++
	}
	if checked < 20 {
		t.Fatalf("only %d admin routes found — route walk is broken", checked)
	}
}
