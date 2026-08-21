package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"interview-chain/internal/transaction"
)

type Block struct {
	Height       int64                `json:"height"`
	Hash         string               `json:"hash"`
	PreviousHash string               `json:"previous_hash"`
	Transactions []transaction.Signed `json:"transactions"`
}

type BlockPage struct {
	Blocks     []Block `json:"blocks"`
	NextHeight int64   `json:"next_height"`
	HasMore    bool    `json:"has_more"`
}

type SubmissionResult struct {
	TransactionID string            `json:"transaction_id"`
	Status        TransactionStatus `json:"status"`
	BlockHeight   *int64            `json:"block_height,omitempty"`
	Duplicate     bool              `json:"duplicate"`
}

type TransactionStatus string

const (
	TransactionPending  TransactionStatus = "pending"
	TransactionIncluded TransactionStatus = "included"
)

type TransactionResult struct {
	Transaction transaction.Signed `json:"transaction"`
	Status      TransactionStatus  `json:"status"`
	BlockHeight *int64             `json:"block_height,omitempty"`
}

type AccountBalance struct {
	Address      string `json:"address"`
	BalanceUnits int64  `json:"balance_units"`
}

func CalculateBlockHash(previous string, height int64, transactionIDs ...string) string {
	var payload strings.Builder
	payload.WriteString(previous)
	payload.WriteByte(0)
	payload.WriteString(strconv.FormatInt(height, 10))
	for _, transactionID := range transactionIDs {
		payload.WriteByte(0)
		payload.WriteString(transactionID)
	}
	sum := sha256.Sum256([]byte(payload.String()))
	return hex.EncodeToString(sum[:])
}
