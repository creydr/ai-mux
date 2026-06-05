package main

import (
	"os"

	"github.com/creydr/ai-mux/cmd/ai-mux/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
