package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"davdeck.dev/davdeck/core/internal/domain"
)

const secretMasterKeySize = 32

// LocalEncryptedSecretStore encrypts provider authentication material using a
// machine-local AES-256-GCM key. The key file is separate from SQLite and is
// restricted to the current account using Unix permissions or Windows ACLs.
type LocalEncryptedSecretStore struct {
	db   *sql.DB
	aead cipher.AEAD
}

func NewLocalEncryptedSecretStore(db *sql.DB, keyPath string) (*LocalEncryptedSecretStore, error) {
	key, err := loadOrCreateSecretKey(keyPath)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create secret cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create secret cipher mode: %w", err)
	}
	return &LocalEncryptedSecretStore{db: db, aead: aead}, nil
}

func (s *LocalEncryptedSecretStore) Get(ctx context.Context, id domain.ID) (domain.DNSProviderSecret, bool, error) {
	var ciphertext []byte
	err := s.db.QueryRowContext(ctx, `SELECT ciphertext FROM dns_provider_secrets WHERE credential_id = ?`, id).Scan(&ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	plaintext, err := s.decrypt(ciphertext)
	if err != nil {
		return nil, false, fmt.Errorf("decrypt DNS provider secret: %w", err)
	}
	var secret domain.DNSProviderSecret
	if err := json.Unmarshal(plaintext, &secret); err != nil {
		return nil, false, fmt.Errorf("decode DNS provider secret: %w", err)
	}
	return secret, true, nil
}

func (s *LocalEncryptedSecretStore) Put(ctx context.Context, id domain.ID, secret domain.DNSProviderSecret, updated domain.Timestamp) error {
	plaintext, err := json.Marshal(map[string]string(secret))
	if err != nil {
		return fmt.Errorf("encode DNS provider secret: %w", err)
	}
	ciphertext, err := s.encrypt(plaintext)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO dns_provider_secrets(credential_id, ciphertext, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(credential_id) DO UPDATE SET ciphertext = excluded.ciphertext, updated_at = excluded.updated_at`, id, ciphertext, updated.String())
	return err
}

func (s *LocalEncryptedSecretStore) Delete(ctx context.Context, id domain.ID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM dns_provider_secrets WHERE credential_id = ?`, id)
	return err
}

func (s *LocalEncryptedSecretStore) encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate secret nonce: %w", err)
	}
	return s.aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (s *LocalEncryptedSecretStore) decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < s.aead.NonceSize() {
		return nil, errors.New("encrypted secret is truncated")
	}
	nonce, body := ciphertext[:s.aead.NonceSize()], ciphertext[s.aead.NonceSize():]
	return s.aead.Open(nil, nonce, body, nil)
}

func loadOrCreateSecretKey(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("secret key path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create secret key directory: %w", err)
	}
	if err := secureSecretKeyDirectory(filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("secure secret key directory: %w", err)
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, errors.New("secret key path must be a regular file")
		}
		key, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read secret master key: %w", err)
		}
		if len(key) != secretMasterKeySize {
			return nil, errors.New("secret master key has invalid length")
		}
		if err := secureSecretKeyFile(path); err != nil {
			return nil, fmt.Errorf("secure secret master key: %w", err)
		}
		return key, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect secret master key: %w", err)
	}
	key := make([]byte, secretMasterKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate secret master key: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create secret master key: %w", err)
	}
	if err := secureSecretKeyFile(path); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("secure secret master key: %w", err)
	}
	if _, err := file.Write(key); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("write secret master key: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("sync secret master key: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close secret master key: %w", err)
	}
	return key, nil
}
