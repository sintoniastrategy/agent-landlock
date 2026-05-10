package agentlandlock

import "fmt"

func shellFields(s string) ([]string, error) {
	var out []string
	var buf []rune
	var quote rune
	escaped := false
	inToken := false

	flush := func() {
		if inToken {
			out = append(out, string(buf))
			buf = nil
			inToken = false
		}
	}

	for _, r := range s {
		if escaped {
			buf = append(buf, r)
			inToken = true
			escaped = false
			continue
		}
		if quote != 0 {
			if r == '\\' && quote == '"' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
				inToken = true
				continue
			}
			buf = append(buf, r)
			inToken = true
			continue
		}
		switch r {
		case '\\':
			escaped = true
			inToken = true
		case '\'', '"':
			quote = r
			inToken = true
		case ' ', '\t', '\n', '\r':
			flush()
		default:
			buf = append(buf, r)
			inToken = true
		}
	}
	if escaped {
		return nil, fmt.Errorf("unfinished escape in config value")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in config value")
	}
	flush()
	return out, nil
}
