package main

import (
	"fmt"
	"os"

	"github.com/proofboard/proofboard/internal/crypto"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/verify_signature BINARY SIGNATURE")
		os.Exit(2)
	}
	binary, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "read binary: %v\n", err)
		os.Exit(1)
	}
	signature, err := os.ReadFile(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "read signature: %v\n", err)
		os.Exit(1)
	}
	if err := crypto.VerifyReleaseSignature(binary, signature); err != nil {
		fmt.Fprintf(os.Stderr, "verify signature: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("verified %s\n", os.Args[1])
}
