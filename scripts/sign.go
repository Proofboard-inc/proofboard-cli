package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run sign.go <private_key_pem> <file_to_sign>")
		os.Exit(1)
	}

	keyFile := os.Args[1]
	targetFile := os.Args[2]

	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		fmt.Printf("Failed to read private key: %v\n", err)
		os.Exit(1)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		fmt.Println("Failed to decode PEM block containing private key")
		os.Exit(1)
	}

	priv, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		fmt.Printf("Failed to parse private key: %v\n", err)
		os.Exit(1)
	}

	targetData, err := os.ReadFile(targetFile)
	if err != nil {
		fmt.Printf("Failed to read target file: %v\n", err)
		os.Exit(1)
	}

	hash := sha256.Sum256(targetData)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, hash[:])
	if err != nil {
		fmt.Printf("Failed to sign: %v\n", err)
		os.Exit(1)
	}

	sigFile := targetFile + ".sig"
	if err := os.WriteFile(sigFile, sig, 0644); err != nil {
		fmt.Printf("Failed to write signature file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully signed %s -> %s\n", targetFile, filepath.Base(sigFile))
}
