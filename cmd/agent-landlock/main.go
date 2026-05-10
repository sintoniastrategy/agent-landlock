package main

import (
	"fmt"
	"os"

	"github.com/rv/agent-landlock/internal/agentlandlock"
)

func main() {
	code, err := agentlandlock.Main(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent-landlock: %v\n", err)
	}
	os.Exit(code)
}
