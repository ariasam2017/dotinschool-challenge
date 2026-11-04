package web

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/dotinschool/dotinschool-challenge/internal/apperr"
)

// Transport policy (docs/security/TLS-POLICY.md):
//
//   - A request is "secure" when it arrived over TLS on this process, or from a
//     trusted proxy that says X-Forwarded-Proto: https. A loopback connection
//     carrying no forwarding headers (a dev run, the container healthcheck, an
//     SSH port-forward) is also accepted — it never crossed a network.
//   - In production every non-secure request is refused.
//   - The admin panel and admin API refuse non-secure requests in every mode
//     (unless ALLOW_INSECURE_ADMIN, which production refuses to start with),
//     and answer only the ADMIN_ALLOWED_CIDRS source networks.

var forwardingHeaders = []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Proto", "X-Real-IP", "CF-Connecting-IP"}

// parseTrustedNets mirrors the list configureProxyTrust hands gin, so the TLS
// decision trusts exactly the same peers the client-IP decision does.
func parseTrustedNets(extra []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, p := range append([]string{"127.0.0.1", "::1"}, extra...) {
		if !strings.Contains(p, "/") {
			if ip := net.ParseIP(p); ip != nil {
				if ip.To4() != nil {
					p += "/32"
				} else {
					p += "/128"
				}
			}
		}
		if _, n, err := net.ParseCIDR(p); err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}

func inNets(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// peerIP is the address of the TCP peer itself (never a header).
func peerIP(c *gin.Context) net.IP {
	return net.ParseIP(c.RemoteIP())
}

// servedOverTLS reports whether the visitor's connection was encrypted: TLS on
// this process, or a trusted proxy reporting https.
func (s *server) servedOverTLS(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	peer := peerIP(c)
	if peer == nil || !inNets(peer, s.trustedNets) {
		return false
	}
	proto := c.GetHeader("X-Forwarded-Proto")
	if i := strings.IndexByte(proto, ','); i >= 0 {
		proto = proto[:i] // the first hop's view is the visitor's
	}
	return strings.EqualFold(strings.TrimSpace(proto), "https")
}

// localPlain is a plain-HTTP request that never left this machine.
func localPlain(c *gin.Context) bool {
	peer := peerIP(c)
	if peer == nil || !peer.IsLoopback() {
		return false
	}
	for _, h := range forwardingHeaders {
		if c.GetHeader(h) != "" {
			return false
		}
	}
	return true
}

func (s *server) isSecure(c *gin.Context) bool {
	return s.servedOverTLS(c) || localPlain(c)
}

// requireSecureTransport refuses every non-secure request in production, so a
// plain-HTTP port that ends up exposed by mistake serves nothing.
func (s *server) requireSecureTransport() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.cfg.Production || s.isSecure(c) {
			c.Next()
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			fail(c, apperr.New(http.StatusForbidden, "TLS_REQUIRED", "این سامانه فقط از طریق اتصال امن HTTPS در دسترس است."))
		} else {
			c.String(http.StatusForbidden, "HTTPS required — این سامانه فقط از طریق HTTPS در دسترس است.")
		}
		c.Abort()
	}
}

// adminNetworkAllowed checks the visitor (c.ClientIP — the same single source
// the rate limits use) against ADMIN_ALLOWED_CIDRS.
func (s *server) adminNetworkAllowed(c *gin.Context) bool {
	if s.cfg.AdminAllowAll || len(s.cfg.AdminAllowedNets) == 0 {
		return true
	}
	ip := net.ParseIP(c.ClientIP())
	return ip != nil && inNets(ip, s.cfg.AdminAllowedNets)
}

// adminChannelErr is why this request may not touch the admin surface, or nil.
// Outside the management network the answer is a plain 404, so the panel's
// existence isn't confirmed to the internet.
func (s *server) adminChannelErr(c *gin.Context) *apperr.Error {
	if !s.adminNetworkAllowed(c) {
		s.log.WithField("ip", c.ClientIP()).WithField("path", c.Request.URL.Path).Warn("admin access denied: outside allowed network")
		return apperr.New(http.StatusNotFound, "NOT_FOUND", "یافت نشد.")
	}
	if !s.cfg.AllowInsecureAdmin && !s.isSecure(c) {
		return apperr.New(http.StatusForbidden, "TLS_REQUIRED", "ورود و کار با پنل مدیریت فقط از طریق اتصال امن HTTPS مجاز است.")
	}
	return nil
}

// adminChannel guards the unauthenticated admin entry points (login).
func (s *server) adminChannel() gin.HandlerFunc {
	return func(c *gin.Context) {
		if e := s.adminChannelErr(c); e != nil {
			fail(c, e)
			c.Abort()
			return
		}
		c.Next()
	}
}

// resolveAdmin is Sessions.ResolveAdmin plus the admin channel rules, for the
// routes that accept an admin token optionally (participant PATCH, config).
func (s *server) resolveAdmin(c *gin.Context) (string, bool) {
	u, ok := s.svc.Sessions.ResolveAdmin(bearerToken(c))
	if !ok || s.adminChannelErr(c) != nil {
		return "", false
	}
	return u, true
}

// longRequest widens the server-wide read/write deadlines for the few routes
// that legitimately move a large body (DB restore/backup, participant import).
func longRequest(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		rc := http.NewResponseController(c.Writer)
		deadline := time.Now().Add(d)
		_ = rc.SetReadDeadline(deadline)
		_ = rc.SetWriteDeadline(deadline)
		c.Next()
	}
}
