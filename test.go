package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func main() {
	hash := sha256.Sum256([]byte("/workspaces/proofboard-cli"))
	fmt.Println(hex.EncodeToString(hash[:]))
}
