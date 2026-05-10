package agentlsm

import (
	"fmt"
	"runtime"

	"github.com/landlock-lsm/go-landlock/landlock"
	llsyscall "github.com/landlock-lsm/go-landlock/landlock/syscall"
)

type SandboxPolicy struct {
	ReadOnlyRoot bool
	Writable     []string
}

func LandlockABIVersion() (int, error) {
	if runtime.GOOS != "linux" {
		return 0, fmt.Errorf("Landlock is Linux-only")
	}
	return llsyscall.LandlockGetABIVersion()
}

func applySandbox(policy SandboxPolicy) error {
	abi, err := LandlockABIVersion()
	if err != nil {
		return exitError(ExitLandlockUnavailable, fmt.Sprintf("Landlock unavailable: %v", err))
	}
	if abi < 3 {
		return exitError(
			ExitLandlockUnavailable,
			fmt.Sprintf("Landlock ABI v%d is too old; agent-lsm requires ABI v3+ to restrict truncation", abi),
		)
	}
	cfg := landlockConfigForABI(abi)
	rules := []landlock.Rule{landlock.RODirs("/").WithIoctlDev()}
	if len(policy.Writable) > 0 {
		rules = append(rules, landlock.RWDirs(policy.Writable...).WithRefer())
	}
	if err := cfg.RestrictPaths(rules...); err != nil {
		return exitError(ExitLandlockUnavailable, fmt.Sprintf("could not enforce Landlock policy: %v", err))
	}
	return nil
}

func landlockConfigForABI(abi int) landlock.Config {
	switch {
	case abi >= 8:
		return landlock.V8
	case abi == 7:
		return landlock.V7
	case abi == 6:
		return landlock.V6
	case abi == 5:
		return landlock.V5
	case abi == 4:
		return landlock.V4
	default:
		return landlock.V3
	}
}
