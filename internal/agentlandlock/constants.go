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
	"/", "/root",
}

var defaultSystemWritablePaths = []string{
	"/bin",
	"/boot",
	"/dev",
	"/etc",
	"/lib",
	"/lib64",
	"/media",
	"/mnt",
	"/opt",
	"/proc",
	"/run",
	"/sbin",
	"/srv",
	"/sys",
	"/tmp",
	"/usr",
	"/var",
}

var agentYoloArgs = map[string][]string{
	"claude": {"--dangerously-skip-permissions"},
	"codex":  {"--dangerously-bypass-approvals-and-sandbox"},
	"gemini": {"--approval-mode", "yolo"},
}

var agentStateDirs = map[string][]string{
	"claude": {".claude", ".local/state/claude"},
	"codex":  {".codex"},
	"gemini": {".gemini"},
}
