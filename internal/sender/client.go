package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"interview-chain/internal/blockchain"
	"interview-chain/internal/transaction"
)

type NodeClient struct {
	baseURL string
	http    *http.Client
}

func NewNodeClient(baseURL string) *NodeClient {
	return &NodeClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{},
	}
}

func (c *NodeClient) Submit(ctx context.Context, tx transaction.Signed) (blockchain.SubmissionResult, error) {
	body, err := json.Marshal(tx)
	if err != nil {
		return blockchain.SubmissionResult{}, fmt.Errorf("encode transaction: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/transactions", bytes.NewReader(body))
	if err != nil {
		return blockchain.SubmissionResult{}, fmt.Errorf("create submit request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return blockchain.SubmissionResult{}, fmt.Errorf("submit transaction: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted && response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return blockchain.SubmissionResult{}, fmt.Errorf("node returned %s: %s", response.Status, strings.TrimSpace(string(message)))
	}
	var result blockchain.SubmissionResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return blockchain.SubmissionResult{}, fmt.Errorf("decode submit response: %w", err)
	}
	return result, nil
}
