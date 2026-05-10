package agentlandlock

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	unit := byte('s')
	last := s[len(s)-1]
	if last < '0' || last > '9' {
		unit = last
		s = strings.TrimSpace(s[:len(s)-1])
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid duration")
	}
	switch unit {
	case 's', 'S':
		return time.Duration(n) * time.Second, nil
	case 'm', 'M':
		return time.Duration(n) * time.Minute, nil
	case 'h', 'H':
		return time.Duration(n) * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid duration unit")
	}
}
