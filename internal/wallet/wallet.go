package wallet

import (
	"encoding/json"
	"fmt"
	"os"
)

type Snapshot struct {
	Accounts []Account `json:"accounts"`
}

type Account struct {
	Address      string `json:"address"`
	Secret       string `json:"secret"`
	BalanceUnits int64  `json:"balance_units"`
}

func Load(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read wallet: %w", err)
	}
	var result Snapshot
	if err := json.Unmarshal(data, &result); err != nil {
		return Snapshot{}, fmt.Errorf("decode wallet: %w", err)
	}
	if len(result.Accounts) == 0 {
		return Snapshot{}, fmt.Errorf("wallet contains no accounts")
	}
	return result, nil
}

func SigningKeys(source Snapshot) map[string]string {
	result := make(map[string]string, len(source.Accounts))
	for _, account := range source.Accounts {
		result[account.Address] = account.Secret
	}
	return result
}
