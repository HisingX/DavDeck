package app

import "golang.org/x/crypto/bcrypt"

const bcryptCost = 12

// BcryptHasher stores Caddy-compatible bcrypt hashes and never plaintext.
type BcryptHasher struct{}

func (BcryptHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	return string(hash), err
}

func (BcryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
