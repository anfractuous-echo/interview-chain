package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"interview-chain/internal/blockchain"
)

type NodeClient struct {
	baseURL string
	http    *http.Client
}

func NewNodeClient(baseURL string) *NodeClient {
	return &NodeClient{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{}}
}

func (c *NodeClient) Blocks(ctx context.Context, after int64, limit int) (blockchain.BlockPage, error) {
	query := url.Values{}
	query.Set("after_height", strconv.FormatInt(after, 10))
	query.Set("limit", strconv.Itoa(limit))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/blocks?"+query.Encode(), nil)
	if err != nil {
		return blockchain.BlockPage{}, err
	}
	response, err := c.http.Do(req)
	if err != nil {
		return blockchain.BlockPage{}, fmt.Errorf("fetch blocks: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return blockchain.BlockPage{}, fmt.Errorf("node returned %s", response.Status)
	}
	defer response.Body.Close()
	var page blockchain.BlockPage
	if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
		return blockchain.BlockPage{}, fmt.Errorf("decode blocks: %w", err)
	}
	return page, nil
}
