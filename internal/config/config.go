// Package config loads runtime configuration from environment variables and a
// local .env file, applying the same defaults the original Node server used.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

// Config is the fully resolved runtime configuration. It is built once at
// startup and treated as read-only afterwards.
type Config struct {
	Root    string // absolute path of the working directory
	DataDir string // where arena.db + ai-config.json live

	Port     string // listen port (":" is prepended by the server)
	UseTLS   bool
	CertPath string
	KeyPath  string

	// How the panel is reached from outside — used only for the addresses
	// printed at start-up (the server itself always binds every interface on
	// Port). Priority: PublicURL > Domain > PublicHost > auto-detected IP.
	Domain     string // DOMAIN — the public (sub)domain, no scheme, no port
	PublicHost string // PUBLIC_HOST — explicit host/IP when there is no domain
	PublicPort string // PUBLIC_PORT — the port published to the outside world
	PublicURL  string // PUBLIC_URL — a full "scheme://host[:port]" override

	InDocker bool

	AdminUsername  string
	AdminPassword  string
	AdminPath      string // normalised, always begins with "/" (e.g. "/admin")
	AdminSubdomain string // lower-case host, no port; "" when disabled

	// TrustedProxies are extra IPs/CIDRs (beyond loopback) whose forwarding
	// headers are believed — the Caddy container's fixed address.
	TrustedProxies []string
	// ClientIPHeader is the ONE forwarding header the visitor's address is read
	// from, and only when the request comes from a trusted proxy. Rate limits,
	// fail2ban and the access log all use that single value.
	ClientIPHeader string

	// Production (APP_ENV=production) turns the transport rules from advice
	// into hard requirements: no plain-HTTP serving, no insecure admin, an
	// explicit admin network allow-list — see Validate.
	Production bool
	// TLSProxy (TLS_PROXY=1): TLS is terminated by a trusted proxy in front
	// (Caddy, Cloudflare Tunnel, a corporate load balancer) that reports the
	// scheme in X-Forwarded-Proto.
	TLSProxy bool
	// AllowInsecureAdmin (ALLOW_INSECURE_ADMIN=1) lets the admin panel work
	// over plain HTTP from a non-loopback address. Development only; refused
	// in production.
	AllowInsecureAdmin bool
	// AdminAllowAll is ADMIN_ALLOWED_CIDRS=any — the admin network restriction
	// is enforced somewhere else (Cloudflare Access, a VPN-only listener).
	AdminAllowAll bool
	// AdminAllowedNets is the parsed ADMIN_ALLOWED_CIDRS: the only source
	// addresses the admin panel and admin API answer. Empty = unrestricted
	// (development only).
	AdminAllowedNets []*net.IPNet
	badAdminCIDRs    []string

	// SMS (ippanel) — every value optional; when APIKey is empty the OTP
	// endpoints report SMS_SEND_FAILED just like the original.
	IppanelAPIKey             string
	IppanelSender             string
	IppanelRegisterPattern    string
	IppanelLoginPattern       string
	IppanelAdminNotifyPattern string
	IppanelAdminPhone         string
	IppanelBaseURL            string // override the gateway endpoint
	IppanelAuthScheme         string // "" (raw) | AccessKey | apikey | Bearer

	// AI fallback (lowest priority; panel DB and data/ai-config.json win).
	AIEnvAPIKey  string
	AIEnvBaseURL string
	AIEnvModel   string

	envPath string // .env file we may append a generated password to
}

const (
	defaultIppanelSender          = "+98pro"
	defaultIppanelRegisterPattern = "f5v9cpylajom6eq"
	defaultIppanelLoginPattern    = "rof25w1ugehwww7"
	defaultIppanelAdminPattern    = "8pntryffb766kiu"
)

var trimSlashes = regexp.MustCompile(`^/+|/+$`)

// Load resolves configuration. root is the process working directory; log is
// used only for the "a random admin password was generated" notice.
func Load(root string, log *logrus.Logger) *Config {
	root, _ = filepath.Abs(root)
	envPath := filepath.Join(root, ".env")
	loadDotEnv(envPath)

	c := &Config{
		Root:    root,
		envPath: envPath,
		DataDir: firstNonEmpty(os.Getenv("DATA_DIR"), filepath.Join(root, "data")),

		CertPath: os.Getenv("SSL_CERT_PATH"),
		KeyPath:  os.Getenv("SSL_KEY_PATH"),

		InDocker: fileExists("/.dockerenv") || os.Getenv("DOTINSCHOOL_DOCKER") == "1",

		AdminUsername: firstNonEmpty(os.Getenv("ADMIN_USERNAME"), "administrator"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),

		IppanelAPIKey:             os.Getenv("IPPANEL_API_KEY"),
		IppanelSender:             firstNonEmpty(os.Getenv("IPPANEL_SENDER"), defaultIppanelSender),
		IppanelRegisterPattern:    firstNonEmpty(os.Getenv("IPPANEL_PATTERN_CODE"), defaultIppanelRegisterPattern),
		IppanelLoginPattern:       firstNonEmpty(os.Getenv("IPPANEL_LOGIN_PATTERN_CODE"), defaultIppanelLoginPattern),
		IppanelAdminNotifyPattern: firstNonEmpty(os.Getenv("IPPANEL_ADMIN_NOTIFY_PATTERN_CODE"), defaultIppanelAdminPattern),
		IppanelAdminPhone:         strings.TrimPrefix(strings.TrimSpace(os.Getenv("IPPANEL_ADMIN_PHONE")), "0"),
		IppanelBaseURL:            strings.TrimSpace(os.Getenv("IPPANEL_BASE_URL")),
		IppanelAuthScheme:         strings.TrimSpace(os.Getenv("IPPANEL_AUTH_SCHEME")),

		AIEnvAPIKey:  strings.TrimSpace(os.Getenv("AI_API_KEY")),
		AIEnvBaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("AI_BASE_URL")), "/"),
		AIEnvModel:   strings.TrimSpace(os.Getenv("AI_MODEL")),
	}

	// TLS only when both files are set and actually present on disk — otherwise
	// plain HTTP, exactly like a raw-IP deployment with no domain.
	c.UseTLS = c.CertPath != "" && c.KeyPath != "" && fileExists(c.CertPath) && fileExists(c.KeyPath)

	c.Port = os.Getenv("PORT")
	if c.Port == "" {
		if c.UseTLS {
			c.Port = "443"
		} else {
			c.Port = "8080"
		}
	}

	adminPath := trimSlashes.ReplaceAllString(firstNonEmpty(os.Getenv("ADMIN_PATH"), "admin"), "")
	c.AdminPath = "/" + adminPath

	sub := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_SUBDOMAIN")))
	if i := strings.LastIndex(sub, ":"); i >= 0 {
		sub = sub[:i]
	}
	c.AdminSubdomain = sub

	for _, p := range strings.Split(os.Getenv("TRUSTED_PROXIES"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			c.TrustedProxies = append(c.TrustedProxies, p)
		}
	}
	c.ClientIPHeader = firstNonEmpty(strings.TrimSpace(os.Getenv("CLIENT_IP_HEADER")), "X-Forwarded-For")

	c.Production = strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
	c.TLSProxy = envBool("TLS_PROXY")
	c.AllowInsecureAdmin = envBool("ALLOW_INSECURE_ADMIN")
	c.AdminAllowAll, c.AdminAllowedNets, c.badAdminCIDRs = parseAllowList(os.Getenv("ADMIN_ALLOWED_CIDRS"))

	c.Domain = strings.TrimSpace(os.Getenv("DOMAIN"))
	c.PublicHost = strings.TrimSpace(os.Getenv("PUBLIC_HOST"))
	c.PublicURL = strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_URL")), "/")
	// The outside port: an explicit override, else the host-published port the
	// Docker stacks pass through (APP_PORT for plain HTTP, HTTPS_PORT for the
	// 3x-ui-style direct-TLS mode), else the port we listen on.
	c.PublicPort = firstNonEmpty(
		strings.TrimSpace(os.Getenv("PUBLIC_PORT")),
		strings.TrimSpace(os.Getenv("APP_PORT")),
	)
	if c.UseTLS {
		c.PublicPort = firstNonEmpty(
			strings.TrimSpace(os.Getenv("PUBLIC_PORT")),
			strings.TrimSpace(os.Getenv("HTTPS_PORT")),
		)
	}

	// No fixed password baked into the binary. Generate a strong one, persist it
	// to .env so it stays stable across restarts, and print it once.
	if c.AdminPassword == "" {
		gen := randomPassword(20)
		c.AdminPassword = gen
		_ = os.Setenv("ADMIN_PASSWORD", gen)
		appendEnv(envPath, "ADMIN_PASSWORD", gen)
		log.Infof("یک رمز عبور تصادفی برای ادمین ساخته شد (در .env هم ذخیره شد): %s", gen)
	}

	return c
}

// Validate reports configuration that must stop the server from starting. In
// production that is every way the panel could end up served without TLS, or
// the admin panel reachable from anywhere: the safe mode must be the only mode,
// not a setting someone has to remember.
func (c *Config) Validate() error {
	var problems []string
	if len(c.badAdminCIDRs) > 0 {
		problems = append(problems, "ADMIN_ALLOWED_CIDRS has invalid entries: "+strings.Join(c.badAdminCIDRs, ", "))
	}
	if c.Production {
		if (c.CertPath != "" || c.KeyPath != "") && !c.UseTLS {
			problems = append(problems, "SSL_CERT_PATH/SSL_KEY_PATH are set but the files are missing — refusing to fall back to plain HTTP")
		}
		if !c.UseTLS && !c.TLSProxy {
			problems = append(problems, "APP_ENV=production needs TLS: set SSL_CERT_PATH and SSL_KEY_PATH, or TLS_PROXY=1 behind a TLS-terminating proxy")
		}
		if c.AllowInsecureAdmin {
			problems = append(problems, "ALLOW_INSECURE_ADMIN is a development switch and is not allowed with APP_ENV=production")
		}
		if !c.AdminAllowAll && len(c.AdminAllowedNets) == 0 {
			problems = append(problems, "APP_ENV=production needs ADMIN_ALLOWED_CIDRS (the management network), or ADMIN_ALLOWED_CIDRS=any when the edge enforces it")
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "; "))
}

// parseAllowList reads a comma-separated list of IPs/CIDRs. "any" means the
// restriction is deliberately delegated elsewhere. A bare IP becomes a /32
// (or /128). Invalid entries are returned so Validate can refuse to start —
// silently dropping a typo would quietly widen access.
func parseAllowList(raw string) (allowAll bool, nets []*net.IPNet, bad []string) {
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		switch {
		case p == "":
			continue
		case strings.EqualFold(p, "any"):
			allowAll = true
			continue
		}
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
		} else {
			bad = append(bad, p)
		}
	}
	return allowAll, nets, bad
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// AIConfigPath is the self-healing sidecar copy of the panel AI settings.
func (c *Config) AIConfigPath() string { return filepath.Join(c.DataDir, "ai-config.json") }

// SMSConfigPath is the self-healing sidecar copy of the panel SMS settings.
func (c *Config) SMSConfigPath() string { return filepath.Join(c.DataDir, "sms-config.json") }

// DBPath is the single SQLite file that holds all state.
func (c *Config) DBPath() string { return filepath.Join(c.DataDir, "arena.db") }

// PublicBaseURL is the "scheme://host[:port]" the panel is reached on from
// outside — for the addresses printed at start-up. It never ends with "/".
// It falls back to the machine's LAN IP, and only to "localhost" when even
// that cannot be determined (a local dev run, typically).
func (c *Config) PublicBaseURL() string {
	if c.PublicURL != "" {
		return c.PublicURL
	}

	scheme := "http"
	if c.UseTLS {
		scheme = "https"
	}

	standardPort := func(p string) bool {
		return (scheme == "http" && (p == "" || p == "80")) ||
			(scheme == "https" && (p == "" || p == "443"))
	}

	// A real domain is the clearest label; keep the port off it unless it is
	// non-standard (e.g. HTTPS on :8443 because :443 was taken).
	if c.Domain != "" {
		host := c.Domain
		if !standardPort(c.PublicPort) {
			host += ":" + c.PublicPort
		}
		return scheme + "://" + host
	}

	host := firstNonEmpty(c.PublicHost, outboundIP(c.InDocker), "localhost")
	port := firstNonEmpty(c.PublicPort, c.Port)
	if !standardPort(port) {
		host += ":" + port
	}
	return scheme + "://" + host
}

// outboundIP returns the primary non-loopback IPv4 address of this host by
// asking the kernel which local address a UDP socket to a public IP would use
// (no packet is actually sent). Inside a bridged container this is the
// container's own address, which is useless to a browser, so it is skipped
// there — PUBLIC_HOST or DOMAIN is expected instead. Returns "" on failure.
func outboundIP(inDocker bool) string {
	if inDocker {
		return ""
	}
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	if a, ok := conn.LocalAddr().(*net.UDPAddr); ok && a.IP != nil && !a.IP.IsLoopback() {
		return a.IP.String()
	}
	return ""
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func fileExists(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

func randomPassword(n int) string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	s := base64.StdEncoding.EncodeToString(buf)
	clean := make([]rune, 0, n)
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			clean = append(clean, r)
		}
		if len(clean) == n {
			break
		}
	}
	return string(clean)
}
