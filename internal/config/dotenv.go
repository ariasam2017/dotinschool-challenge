package config

import (
	"bufio"
	"os"
	"strings"
)

// loadDotEnv reads KEY=VALUE lines from file into the process environment
// without overriding anything already set (so a systemd EnvironmentFile or a
// Docker `environment:` block always wins over a local .env). No dependency on
// a dotenv package — the format we accept is deliberately tiny.
func loadDotEnv(file string) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if key == "" {
			continue
		}
		if _, present := os.LookupEnv(key); !present {
			_ = os.Setenv(key, val)
		}
	}
}

// appendEnv appends a single KEY=VALUE line to the .env file. Best effort: on a
// read-only filesystem the generated value simply lives for this process only,
// exactly as the original server behaved.
func appendEnv(file, key, val string) {
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString("\n" + key + "=" + val + "\n")
}
