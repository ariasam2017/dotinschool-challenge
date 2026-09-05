// Package config loads runtime configuration from environment variables and a
// local .env file, applying the same defaults the original Node server used.
package config

import (
	"crypto/rand"
	"encoding/base64"
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

	InDocker bool

	AdminUsername  string
	AdminPassword  string
	AdminPath      string // normalised, always begins with "/" (e.g. "/admin")
	AdminSubdomain string // lower-case host, no port; "" when disabled

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

// AIConfigPath is the self-healing sidecar copy of the panel AI settings.
func (c *Config) AIConfigPath() string { return filepath.Join(c.DataDir, "ai-config.json") }

// SMSConfigPath is the self-healing sidecar copy of the panel SMS settings.
func (c *Config) SMSConfigPath() string { return filepath.Join(c.DataDir, "sms-config.json") }

// DBPath is the single SQLite file that holds all state.
func (c *Config) DBPath() string { return filepath.Join(c.DataDir, "arena.db") }

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
