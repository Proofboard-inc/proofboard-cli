package main

import (
	"context"
	"os"

	"github.com/proofboard/proofboard/internal/commands"
)

func main() {
	ctx := context.Background()
	if err := commands.Execute(ctx, os.Args[1:]); err != nil {
		os.Exit(1)
	}
}
