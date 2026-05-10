package main

import (
	"fmt"
	"os"

	"github.com/rv/agent-lsm/internal/agentlsm"
)

func main() {
	code, err := agentlsm.Main(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent-lsm: %v\n", err)
	}
	os.Exit(code)
}
