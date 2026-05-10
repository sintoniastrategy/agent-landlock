package agentlsm

import (
	"fmt"
	"strings"
)

type CommonOptions struct {
	Dir          string
	RuntimeGrant []string
	DryRun       bool
	Force        bool
	NoYolo       bool
	NoAgentState bool
	Verbose      bool
	Quiet        bool
	Timeout      string
}

type Invocation struct {
	Command string
	Agent   string
	Args    []string
	Path    string
	Cleanup bool
	Heal    bool
	Undo    bool
	Common  CommonOptions
}

func ParseArgs(argv []string, cfg Config) (Invocation, error) {
	common, rest, err := consumeCommon(argv, CommonOptions{})
	if err != nil {
		return Invocation{}, err
	}
	if len(rest) == 0 {
		return Invocation{Command: "agent", Agent: cfg.DefaultAgent, Common: common}, nil
	}
	first := rest[0]
	if KnownAgents[first] {
		return parseAgent(first, rest[1:], common)
	}
	if !knownSubcommands[first] {
		return Invocation{
			Command: "agent",
			Agent:   cfg.DefaultAgent,
			Args:    rest,
			Common:  common,
		}, nil
	}
	switch first {
	case "run":
		return parseRun(rest[1:], common)
	case "shell":
		return parseShell(rest[1:], common)
	case "grant", "revoke", "status", "doctor":
		return parsePathCommand(first, rest[1:], common)
	case "grants", "help", "bootstrap":
		c, tail, err := consumeCommon(rest[1:], common)
		if err != nil {
			return Invocation{}, err
		}
		if first == "grants" {
			filtered := tail[:0]
			cleanup := false
			for _, item := range tail {
				if item == "--cleanup" {
					cleanup = true
					continue
				}
				filtered = append(filtered, item)
			}
			tail = filtered
			if len(tail) > 0 {
				return Invocation{}, exitError(ExitUsage, fmt.Sprintf("%s: unexpected argument: %s", first, tail[0]))
			}
			return Invocation{Command: first, Cleanup: cleanup, Common: c}, nil
		}
		if first == "bootstrap" {
			filtered := tail[:0]
			undo := false
			for _, item := range tail {
				if item == "--undo" {
					undo = true
					continue
				}
				filtered = append(filtered, item)
			}
			tail = filtered
			if len(tail) > 0 {
				return Invocation{}, exitError(ExitUsage, fmt.Sprintf("%s: unexpected argument: %s", first, tail[0]))
			}
			return Invocation{Command: first, Undo: undo, Common: c}, nil
		}
		if len(tail) > 0 {
			return Invocation{}, exitError(ExitUsage, fmt.Sprintf("%s: unexpected argument: %s", first, tail[0]))
		}
		return Invocation{Command: first, Common: c}, nil
	default:
		return Invocation{}, exitError(ExitUsage, fmt.Sprintf("unknown command: %s", first))
	}
}

func parseAgent(agent string, argv []string, common CommonOptions) (Invocation, error) {
	c, rest, err := consumeCommon(argv, common)
	if err != nil {
		return Invocation{}, err
	}
	if len(rest) > 0 && rest[0] == "--" {
		rest = rest[1:]
	}
	return Invocation{Command: "agent", Agent: agent, Args: rest, Common: c}, nil
}

func parseRun(argv []string, common CommonOptions) (Invocation, error) {
	c, rest, err := consumeCommon(argv, common)
	if err != nil {
		return Invocation{}, err
	}
	if len(rest) > 0 && rest[0] == "--" {
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return Invocation{}, exitError(ExitUsage, "usage: agent-lsm run [-- CMD ARGS]")
	}
	return Invocation{Command: "run", Args: rest, Common: c}, nil
}

func parseShell(argv []string, common CommonOptions) (Invocation, error) {
	c, rest, err := consumeCommon(argv, common)
	if err != nil {
		return Invocation{}, err
	}
	if len(rest) > 1 {
		return Invocation{}, exitError(ExitUsage, "usage: agent-lsm shell [DIR]")
	}
	if len(rest) == 1 {
		c.Dir = rest[0]
	}
	return Invocation{Command: "shell", Common: c}, nil
}

func parsePathCommand(name string, argv []string, common CommonOptions) (Invocation, error) {
	c, rest, err := consumeCommon(argv, common)
	if err != nil {
		return Invocation{}, err
	}
	heal := false
	if name == "doctor" {
		filtered := rest[:0]
		for _, item := range rest {
			if item == "--heal" {
				heal = true
				continue
			}
			filtered = append(filtered, item)
		}
		rest = filtered
	}
	if len(rest) > 1 {
		return Invocation{}, exitError(ExitUsage, fmt.Sprintf("%s: unexpected argument: %s", name, rest[1]))
	}
	path := ""
	if len(rest) == 1 {
		path = rest[0]
	}
	return Invocation{Command: name, Path: path, Heal: heal, Common: c}, nil
}

func consumeCommon(argv []string, common CommonOptions) (CommonOptions, []string, error) {
	args := append([]string(nil), argv...)
	for len(args) > 0 {
		token := args[0]
		if token == "--" {
			return common, args, nil
		}
		if !strings.HasPrefix(token, "-") || token == "-" {
			return common, args, nil
		}
		valueFlag, value, ok := splitValueFlag(token)
		if ok {
			if err := applyValueFlag(&common, valueFlag, value); err != nil {
				return common, nil, err
			}
			args = args[1:]
			continue
		}
		switch token {
		case "-d", "--dir", "-g", "--grant", "--timeout":
			if len(args) < 2 {
				return common, nil, exitError(ExitUsage, fmt.Sprintf("%s requires a value", token))
			}
			if err := applyValueFlag(&common, token, args[1]); err != nil {
				return common, nil, err
			}
			args = args[2:]
		case "--dry-run":
			common.DryRun = true
			args = args[1:]
		case "--force":
			common.Force = true
			args = args[1:]
		case "--no-yolo":
			common.NoYolo = true
			args = args[1:]
		case "--no-agent-state":
			common.NoAgentState = true
			args = args[1:]
		case "-v", "--verbose":
			common.Verbose = true
			args = args[1:]
		case "-q", "--quiet":
			common.Quiet = true
			args = args[1:]
		default:
			return common, args, nil
		}
	}
	return common, nil, nil
}

func splitValueFlag(token string) (string, string, bool) {
	for _, flag := range []string{"--dir", "--grant", "--timeout"} {
		prefix := flag + "="
		if strings.HasPrefix(token, prefix) {
			return flag, strings.TrimPrefix(token, prefix), true
		}
	}
	return "", "", false
}

func applyValueFlag(common *CommonOptions, flag, value string) error {
	switch flag {
	case "-d", "--dir":
		common.Dir = value
	case "-g", "--grant":
		common.RuntimeGrant = append(common.RuntimeGrant, value)
	case "--timeout":
		common.Timeout = value
	default:
		return exitError(ExitUsage, fmt.Sprintf("unknown option: %s", flag))
	}
	return nil
}
