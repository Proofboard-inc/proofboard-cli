package main

import (
	"fmt"
	"github.com/proofboard/proofboard/internal/crypto"
)

func main() {
	orgHash := crypto.SHA256("github:Proofboard-inc")
	repoHash := crypto.SHA256("github:Proofboard-inc/proofboard-cli")
	fmt.Printf("orgHash: %s\n", orgHash)
	fmt.Printf("repoHash: %s\n", repoHash)
}
