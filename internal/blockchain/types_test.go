package blockchain

import "testing"

func TestCalculateBlockHashIncludesOrderedTransactionIDs(t *testing.T) {
	t.Parallel()
	empty := CalculateBlockHash("previous", 7)
	firstThenSecond := CalculateBlockHash("previous", 7, "first", "second")
	secondThenFirst := CalculateBlockHash("previous", 7, "second", "first")
	if empty == firstThenSecond {
		t.Fatal("empty and non-empty blocks have the same hash")
	}
	if firstThenSecond == secondThenFirst {
		t.Fatal("transaction order does not affect the block hash")
	}
	if firstThenSecond != CalculateBlockHash("previous", 7, "first", "second") {
		t.Fatal("block hash is not deterministic")
	}
}
