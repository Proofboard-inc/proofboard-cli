package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/proofboard/proofboard/internal/api"
)

const deviceKeyFileMode os.FileMode = 0o600

type DeviceKeyStore struct {
	homeDir string
}

type DeviceKeyRecord struct {
	DeviceKeyID string `json:"deviceKeyId,omitempty"`
	PublicKey   string `json:"publicKey"`
	PrivateKey  string `json:"privateKey"`
}

func NewDeviceKeyStore(homeDir string) DeviceKeyStore {
	return DeviceKeyStore{homeDir: homeDir}
}

func (s DeviceKeyStore) Path() string {
	return filepath.Join(s.homeDir, ".proofboard", "device.key")
}

func (s DeviceKeyStore) Load(ctx context.Context) (DeviceKeyRecord, error) {
	if err := ctx.Err(); err != nil {
		return DeviceKeyRecord{}, fmt.Errorf("load device key: %w", err)
	}
	data, err := os.ReadFile(s.Path())
	if err != nil {
		return DeviceKeyRecord{}, fmt.Errorf("read device key: %w", err)
	}
	var record DeviceKeyRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return DeviceKeyRecord{}, fmt.Errorf("decode device key: %w", err)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create device key directory: %w", err)
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal device key: %w", err)
	}
	if err := os.WriteFile(path, data, deviceKeyFileMode); err != nil {
		return fmt.Errorf("write device key: %w", err)
	}
	return nil
}

func (s DeviceKeyStore) Ensure(ctx context.Context, client api.Client, token string, rotate bool) (DeviceKeyRecord, error) {
	record, err := s.Load(ctx)
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
		return record, nil
	}

	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return DeviceKeyRecord{}, fmt.Errorf("generate device key seed: %w", err)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	record = DeviceKeyRecord{
		PublicKey:   base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey:  base64.StdEncoding.EncodeToString(privateKey),
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
	privateKeyBytes, err := base64.StdEncoding.DecodeString(record.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("decode device private key: %w", err)
	}
	if len(privateKeyBytes) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("decode device private key: invalid size %d", len(privateKeyBytes))
	}
	signature := ed25519.Sign(ed25519.PrivateKey(privateKeyBytes), payload)
	return base64.StdEncoding.EncodeToString(signature), nil
}

func (s DeviceKeyStore) register(ctx context.Context, client api.Client, token string, record DeviceKeyRecord) (DeviceKeyRecord, error) {
	if token == "" {
		return DeviceKeyRecord{}, errors.New("missing auth token")
	}
	publicKey, err := base64.StdEncoding.DecodeString(record.PublicKey)
	if err != nil {
		return DeviceKeyRecord{}, fmt.Errorf("decode device public key: %w", err)
	}
	resp, err := client.RegisterDeviceKey(ctx, token, base64.StdEncoding.EncodeToString(publicKey))
	if err != nil {
		return DeviceKeyRecord{}, err
	}
	if resp.DeviceKeyID == "" {
		return DeviceKeyRecord{}, errors.New("empty device key id")
	}
	record.DeviceKeyID = resp.DeviceKeyID
	return record, nil
}
