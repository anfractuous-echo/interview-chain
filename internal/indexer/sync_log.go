package indexer

import (
	"log"

	"interview-chain/internal/blockchain"
)

func logIndexedPage(page blockchain.BlockPage, checkpoint int64) {
	firstHeight := page.Blocks[0].Height
	lastHeight := page.Blocks[len(page.Blocks)-1].Height
	transactionCount := 0
	for _, block := range page.Blocks {
		transactionCount += len(block.Transactions)
	}

	if page.HasMore {
		log.Printf(
			"indexed range=%d..%d batch_blocks=%d batch_transactions=%d stored_checkpoint=%d node_cursor=%d caught_up=false",
			firstHeight,
			lastHeight,
			len(page.Blocks),
			transactionCount,
			checkpoint,
			page.NextHeight,
		)
		return
	}
	log.Printf(
		"indexed range=%d..%d batch_blocks=%d batch_transactions=%d stored_checkpoint=%d chain_height=%d caught_up=true",
		firstHeight,
		lastHeight,
		len(page.Blocks),
		transactionCount,
		checkpoint,
		page.NextHeight,
	)
}
