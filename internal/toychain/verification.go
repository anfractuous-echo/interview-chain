package toychain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"interview-chain/internal/transaction"
)

func calculateExpectedSignature(tx transaction.Signed, signingKey string) (string, error) {
	payload, err := transaction.CanonicalPayload(tx)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = h.Write([]byte(signingKey))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(payload)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func calculateExpectedID(tx transaction.Signed) (string, error) {
	payload, err := transaction.CanonicalPayload(tx)
	if err != nil {
		return "", err
	}
	signature, err := hex.DecodeString(tx.Signature)
	if err != nil {
		return "", fmt.Errorf("decode signature: %w", err)
	}
	h := sha256.New()
	_, _ = h.Write(payload)
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(signature)
	return hex.EncodeToString(h.Sum(nil)), nil
}
