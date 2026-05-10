package agentlandlock

const (
	ToolName  = "agent-landlock"
	StateName = "agent-landlock"
	EnvPrefix = "AGENT_LANDLOCK_"
)

var KnownAgents = map[string]bool{
	"claude": true,
	"codex":  true,
	"gemini": true,
}

var knownSubcommands = map[string]bool{
	"bootstrap": true,
	"doctor":    true,
	"grant":     true,
	"grants":    true,
	"help":      true,
	"revoke":    true,
	"run":       true,
	"shell":     true,
	"status":    true,
	"claude":    true,
	"codex":     true,
	"gemini":    true,
}

var defaultSafetyDenyPaths = []string{
	"/", "/etc", "/var", "/usr", "/opt", "/boot", "/dev", "/proc",
	"/sys", "/root",
}

var agentYoloArgs = map[string][]string{
	"claude": {"--dangerously-skip-permissions"},
	"codex":  {"--dangerously-bypass-approvals-and-sandbox"},
	"gemini": {"--approval-mode", "yolo"},
}

var agentStateDirs = map[string][]string{
	"claude": {".claude"},
	"codex":  {".codex"},
	"gemini": {".gemini"},
}
