package agentlsm

import (
	"os"
	"strings"
)

func runtimeEnv(cfg Config) (map[string]string, error) {
	env := map[string]string{}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	if strings.TrimSpace(cfg.ExtraEnv) != "" {
		fields, err := shellFields(cfg.ExtraEnv)
		if err != nil {
			return nil, err
		}
		for _, field := range fields {
			key, value, ok := strings.Cut(field, "=")
			if ok && key != "" {
				env[key] = value
			}
		}
	}
	return env, nil
}

func envList(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for key, value := range env {
		out = append(out, key+"="+value)
	}
	return out
}
