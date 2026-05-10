package agentlandlock

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	SafetyDenyPaths []string
	ExtraEnv        string
}

func DefaultConfig() Config {
	return Config{
		SafetyDenyPaths: append([]string(nil), defaultSafetyDenyPaths...),
	}
}

func ParseConfigText(text string) map[string]string {
	out := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !isConfigKey(key) {
			continue
		}
		if len(value) >= 2 && value[0] == value[len(value)-1] &&
			(value[0] == '\'' || value[0] == '"') {
			value = value[1 : len(value)-1]
		}
		out[key] = value
	}
	return out
}

func isConfigKey(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if r < 'A' || r > 'Z' {
				return false
			}
			continue
		}
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func LoadConfig() Config {
	cfg := DefaultConfig()
	for _, path := range configPaths() {
		data, err := os.ReadFile(path)
		if err == nil {
			applyConfig(&cfg, ParseConfigText(string(data)))
		}
	}
	envKV := map[string]string{}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok || !strings.HasPrefix(key, EnvPrefix) {
			continue
		}
		envKV[strings.TrimPrefix(key, EnvPrefix)] = value
	}
	applyConfig(&cfg, envKV)
	return cfg
}

func configPaths() []string {
	paths := []string{filepath.Join(string(filepath.Separator), "etc", StateName, "config")}
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		paths = append(paths, filepath.Join(base, StateName, "config"))
	} else if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", StateName, "config"))
	}
	return paths
}

func applyConfig(cfg *Config, kv map[string]string) {
	if v, ok := kv["SAFETY_DENY_PATHS"]; ok {
		if fields, err := shellFields(v); err == nil {
			cfg.SafetyDenyPaths = fields
		}
	}
	if v, ok := kv["EXTRA_ENV"]; ok {
		cfg.ExtraEnv = v
	}
	if v, ok := kv["AGENT_LANDLOCK_EXTRA_ENV"]; ok {
		cfg.ExtraEnv = v
	}
}
