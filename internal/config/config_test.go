package config

import (
	"strings"
	"testing"
)

func TestParseAllowList(t *testing.T) {
	all, nets, bad := parseAllowList(" 10.0.0.0/8, 192.168.1.7 ,::1, nope ,")
	if all {
		t.Fatal("allowAll set without 'any'")
	}
	if len(nets) != 3 {
		t.Fatalf("parsed %d nets, want 3", len(nets))
	}
	if got := nets[1].String(); got != "192.168.1.7/32" {
		t.Errorf("bare IP became %q, want 192.168.1.7/32", got)
	}
	if len(bad) != 1 || bad[0] != "nope" {
		t.Errorf("bad = %v, want [nope]", bad)
	}
	if all, _, _ := parseAllowList("any"); !all {
		t.Error("'any' not recognised")
	}
}

func TestValidateProduction(t *testing.T) {
	ok := Config{Production: true, TLSProxy: true, AdminAllowAll: true}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid production config rejected: %v", err)
	}

	cases := map[string]struct {
		cfg  Config
		want string
	}{
		"no TLS":         {Config{Production: true, AdminAllowAll: true}, "needs TLS"},
		"missing cert":   {Config{Production: true, TLSProxy: true, AdminAllowAll: true, CertPath: "/nope.pem", KeyPath: "/nope.key"}, "files are missing"},
		"insecure admin": {Config{Production: true, TLSProxy: true, AdminAllowAll: true, AllowInsecureAdmin: true}, "ALLOW_INSECURE_ADMIN"},
		"open admin":     {Config{Production: true, TLSProxy: true}, "ADMIN_ALLOWED_CIDRS"},
		"bad CIDR (dev)": {Config{badAdminCIDRs: []string{"x"}}, "invalid entries"},
	}
	for name, tc := range cases {
		err := tc.cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to mention %q", name, err, tc.want)
		}
	}

	if err := (&Config{}).Validate(); err != nil {
		t.Errorf("development defaults rejected: %v", err)
	}
}
