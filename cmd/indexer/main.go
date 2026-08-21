package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"interview-chain/internal/indexer"
)

type config struct {
	nodeURL      string
	databasePath string
	pageSize     int
}

func parseConfig(args []string) (config, error) {
	var cfg config
	flags := flag.NewFlagSet("indexer", flag.ContinueOnError)
	flags.StringVar(&cfg.nodeURL, "node", "http://localhost:8080", "toy blockchain URL")
	flags.StringVar(&cfg.databasePath, "db", ".data/indexer.db", "indexer SQLite path")
	flags.IntVar(&cfg.pageSize, "page-size", 100, "blocks per request")
	if err := flags.Parse(args); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	database, err := indexer.OpenSQLiteStore(cfg.databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	checkpoint, err := database.Checkpoint(ctx)
	if err != nil {
		log.Fatal(err)
	}
	storedBlocks, storedTransactions, err := database.Counts(ctx)
	if err != nil {
		log.Fatal(err)
	}
	service := indexer.NewService(database, indexer.NewNodeClient(cfg.nodeURL), cfg.pageSize)
	log.Printf(
		"indexer started db=%s stored_checkpoint=%d stored_blocks=%d stored_transactions=%d fetch_limit=%d_blocks_per_request",
		cfg.databasePath,
		checkpoint,
		storedBlocks,
		storedTransactions,
		cfg.pageSize,
	)
	if err := service.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
