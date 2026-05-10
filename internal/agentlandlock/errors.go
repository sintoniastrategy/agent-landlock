package agentlandlock

const (
	ExitOK                  = 0
	ExitGeneric             = 1
	ExitUsage               = 2
	ExitSafety              = 3
	ExitMissingDependency   = 4
	ExitLandlockUnavailable = 5
)

type ExitError struct {
	Code int
	Msg  string
}

func (e *ExitError) Error() string {
	return e.Msg
}

func exitError(code int, msg string) *ExitError {
	return &ExitError{Code: code, Msg: msg}
}
