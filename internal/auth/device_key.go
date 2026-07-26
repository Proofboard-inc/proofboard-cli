package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/zalando/go-keyring"
)

const deviceKeyFileMode os.FileMode = 0o600
const (
	deviceKeyKeychainService = "Proofboard Career Agent"
	deviceKeyKeychainAccount = "device-signing-key"
	deviceKeyAlgorithm       = "ECDSA_P256_SHA256"
)

type deviceKeySecretStore interface {
	Get(service, account string) (string, error)
	Set(service, account, value string) error
}

type systemDeviceKeySecretStore struct{}

func (systemDeviceKeySecretStore) Get(service, account string) (string, error) {
	return keyring.Get(service, account)
}

func (systemDeviceKeySecretStore) Set(service, account, value string) error {
	return keyring.Set(service, account, value)
}

type DeviceKeyStore struct {
	homeDir     string
	secretStore deviceKeySecretStore
}

type DeviceKeyRecord struct {
	DeviceKeyID string `json:"deviceKeyId,omitempty"`
	Algorithm   string `json:"algorithm,omitempty"`
	PublicKey   string `json:"publicKey"`
	PrivateKey  string `json:"privateKey"`
}

func NewDeviceKeyStore(homeDir string) DeviceKeyStore {
	return DeviceKeyStore{homeDir: homeDir, secretStore: systemDeviceKeySecretStore{}}
}

func (s DeviceKeyStore) Path() string {
	return filepath.Join(s.homeDir, ".proofboard", "device.key")
}

func (s DeviceKeyStore) Load(ctx context.Context) (DeviceKeyRecord, error) {
	if err := ctx.Err(); err != nil {
		return DeviceKeyRecord{}, fmt.Errorf("load device key: %w", err)
	}
	if os.Getenv("PROOFBOARD_DISABLE_KEYCHAIN") != "1" && s.secretStore != nil {
		if secret, err := s.secretStore.Get(deviceKeyKeychainService, deviceKeyKeychainAccount); err == nil {
			record, decodeErr := decodeDeviceKeyRecord([]byte(secret))
			if decodeErr != nil {
				return DeviceKeyRecord{}, fmt.Errorf("decode device key from OS keychain: %w", decodeErr)
			}
			return record, nil
		}
	}
	data, err := os.ReadFile(s.Path())
	if err != nil {
		return DeviceKeyRecord{}, fmt.Errorf("read device key: %w", err)
	}
	record, err := decodeDeviceKeyRecord(data)
	if err != nil {
		return DeviceKeyRecord{}, fmt.Errorf("decode device key: %w", err)
	}
	return record, nil
}

func decodeDeviceKeyRecord(data []byte) (DeviceKeyRecord, error) {
	var record DeviceKeyRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return DeviceKeyRecord{}, err
	}
	if record.PublicKey == "" || record.PrivateKey == "" {
		return DeviceKeyRecord{}, errors.New("device key record missing key material")
	}
	return record, nil
}

func (s DeviceKeyStore) Save(ctx context.Context, record DeviceKeyRecord) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save device key: %w", err)
	}
	if record.PublicKey == "" || record.PrivateKey == "" {
		return errors.New("device key record missing key material")
	}
	path := s.Path()
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create device key directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return fmt.Errorf("secure device key directory: %w", err)
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal device key: %w", err)
	}
	if os.Getenv("PROOFBOARD_DISABLE_KEYCHAIN") != "1" && s.secretStore != nil {
		if err := s.secretStore.Set(deviceKeyKeychainService, deviceKeyKeychainAccount, string(data)); err == nil {
			// A successful keychain write supersedes the file fallback. Remove
			// any migrated plaintext copy so only the OS-protected secret
			// remains.
			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("remove migrated device key fallback: %w", removeErr)
			}
			return nil
		}
	}
	if err := os.WriteFile(path, data, deviceKeyFileMode); err != nil {
		return fmt.Errorf("write device key: %w", err)
	}
	if err := os.Chmod(path, deviceKeyFileMode); err != nil {
		return fmt.Errorf("secure device key file: %w", err)
	}
	return nil
}

func (s DeviceKeyStore) Ensure(ctx context.Context, client api.Client, token string, rotate bool) (DeviceKeyRecord, error) {
	record, err := s.Load(ctx)
	if err == nil && record.Algorithm != deviceKeyAlgorithm {
		// Migrate pre-v1.9 Ed25519 installations to the ECDSA P-256 format
		// required by the deployed sync verifier.
		rotate = true
	}
	if err == nil && !rotate {
		if record.DeviceKeyID == "" {
			registered, regErr := s.register(ctx, client, token, record)
			if regErr != nil {
				return DeviceKeyRecord{}, regErr
			}
			record = registered
			if err := s.Save(ctx, record); err != nil {
				return DeviceKeyRecord{}, err
			}
		}
		// Save also migrates a legacy file into the OS keychain when one is
		// available. The record and key ID are otherwise unchanged.
		if err := s.Save(ctx, record); err != nil {
			return DeviceKeyRecord{}, err
		}
		return record, nil
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return DeviceKeyRecord{}, fmt.Errorf("generate ECDSA P-256 device key: %w", err)
	}
	privateKeyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return DeviceKeyRecord{}, fmt.Errorf("marshal device private key: %w", err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return DeviceKeyRecord{}, fmt.Errorf("marshal device public key: %w", err)
	}
	record = DeviceKeyRecord{
		Algorithm:  deviceKeyAlgorithm,
		PublicKey:  string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER})),
		PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateKeyDER})),
	}
	registered, regErr := s.register(ctx, client, token, record)
	if regErr != nil {
		return DeviceKeyRecord{}, regErr
	}
	record = registered
	if err := s.Save(ctx, record); err != nil {
		return DeviceKeyRecord{}, err
	}
	return record, nil
}

func (s DeviceKeyStore) RegisterIfNeeded(ctx context.Context, client api.Client, token string, record DeviceKeyRecord) (DeviceKeyRecord, error) {
	if record.DeviceKeyID != "" {
		return record, nil
	}
	registered, err := s.register(ctx, client, token, record)
	if err != nil {
		return DeviceKeyRecord{}, err
	}
	if err := s.Save(ctx, registered); err != nil {
		return DeviceKeyRecord{}, err
	}
	return registered, nil
}

func (s DeviceKeyStore) Sign(ctx context.Context, payload []byte) (string, error) {
	record, err := s.Load(ctx)
	if err != nil {
		return "", err
	}
	if record.Algorithm != deviceKeyAlgorithm {
		return "", fmt.Errorf("unsupported device key algorithm %q", record.Algorithm)
	}
	block, _ := pem.Decode([]byte(record.PrivateKey))
	if block == nil || block.Type != "EC PRIVATE KEY" {
		return "", errors.New("decode device private key PEM")
	}
	privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse device private key: %w", err)
	}
	if privateKey.Curve != elliptic.P256() {
		return "", errors.New("device private key is not ECDSA P-256")
	}
	digest := sha256.Sum256(payload)
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign sync payload: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func (s DeviceKeyStore) register(ctx context.Context, client api.Client, token string, record DeviceKeyRecord) (DeviceKeyRecord, error) {
	if token == "" {
		return DeviceKeyRecord{}, errors.New("missing auth token")
	}
	if record.Algorithm != deviceKeyAlgorithm {
		return DeviceKeyRecord{}, fmt.Errorf("unsupported device key algorithm %q", record.Algorithm)
	}
	block, _ := pem.Decode([]byte(record.PublicKey))
	if block == nil || block.Type != "PUBLIC KEY" {
		return DeviceKeyRecord{}, errors.New("decode device public key PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return DeviceKeyRecord{}, fmt.Errorf("parse device public key: %w", err)
	}
	publicKey, ok := parsed.(*ecdsa.PublicKey)
	if !ok || publicKey.Curve != elliptic.P256() {
		return DeviceKeyRecord{}, errors.New("device public key is not ECDSA P-256")
	}
	resp, err := client.RegisterDeviceKey(ctx, token, record.PublicKey)
	if err != nil {
		return DeviceKeyRecord{}, err
	}
	if resp.DeviceKeyID == "" {
		return DeviceKeyRecord{}, errors.New("empty device key id")
	}
	record.DeviceKeyID = resp.DeviceKeyID
	return record, nil
}
