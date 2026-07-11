package crypto

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

const ReleasePublicKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEdYPsxqaryQ9bQI3G3hQpsmyrTGs0
nKxvQXQC+nAK+EsNF6VEofCYuX42bTeooKLR1Ol+Eh3NhWErh4tfSkH1mA==
-----END PUBLIC KEY-----`

func VerifyReleaseSignature(data, signature []byte) error {
	block, _ := pem.Decode([]byte(ReleasePublicKey))
	if block == nil {
		return errors.New("failed to decode public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("not ECDSA public key")
	}

	hash := sha256.Sum256(data)
	if !ecdsa.VerifyASN1(ecdsaPub, hash[:], signature) {
		return errors.New("invalid signature")
	}
	return nil
}
