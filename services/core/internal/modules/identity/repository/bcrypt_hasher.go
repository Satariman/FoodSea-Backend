package repository

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const DefaultBcryptCost = 12

type BcryptHasher struct {
	cost int
}

func NewBcryptHasher() *BcryptHasher {
	return &BcryptHasher{cost: DefaultBcryptCost}
}

func NewBcryptHasherWithCost(cost int) *BcryptHasher {
	return &BcryptHasher{cost: cost}
}

func (h *BcryptHasher) Hash(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(hash), nil
}

func (h *BcryptHasher) Verify(hash, plain string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		return fmt.Errorf("bcrypt verify: %w", err)
	}
	return nil
}
