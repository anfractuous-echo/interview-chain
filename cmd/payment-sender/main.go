package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"interview-chain/internal/paymentstore"
	"interview-chain/internal/sender"
	"interview-chain/internal/wallet"
)

type config struct {
	nodeURL      string
	databasePath string
	walletPath   string
	paymentsPath string
}

func parseConfig(args []string) (config, error) {
	var cfg config
	flags := flag.NewFlagSet("payment-sender", flag.ContinueOnError)
	flags.StringVar(&cfg.nodeURL, "node", "http://localhost:8080", "toy blockchain URL")
	flags.StringVar(&cfg.databasePath, "db", ".data/sender.db", "sender SQLite path")
	flags.StringVar(&cfg.walletPath, "wallet", "input-data/wallet.json", "wallet input path")
	flags.StringVar(&cfg.paymentsPath, "payments", "input-data/payments.jsonl", "payments JSONL path")
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	walletData, err := wallet.Load(cfg.walletPath)
	if err != nil {
		log.Fatal(err)
	}
	database, err := paymentstore.OpenSQLite(cfg.databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	if err := sender.NewFileImporter(database).ImportFile(ctx, cfg.paymentsPath); err != nil {
		log.Fatal(err)
	}
	processor := sender.NewProcessor(database, sender.NewNodeClient(cfg.nodeURL), wallet.SigningKeys(walletData))
	const batchSize = 100
	var processed int
	var firstProcessingError error
	lastID := ""
	for {
		payments, err := database.ListReceivedPayments(ctx, lastID, batchSize)
		if err != nil {
			log.Fatal(err)
		}
		if len(payments) == 0 {
			break
		}
		for _, item := range payments {
			lastID = item.ID
			if err := processor.ProcessPayment(ctx, item); err != nil {
				if firstProcessingError == nil {
					firstProcessingError = fmt.Errorf("process payment %s: %w", item.ID, err)
				}
				log.Printf("payment_id=%s result=failed error=%q", item.ID, err)
				continue
			}
			processed++
		}
		if len(payments) < batchSize {
			break
		}
	}
	log.Printf("processed=%d", processed)
	if firstProcessingError != nil {
		log.Fatal(firstProcessingError)
	}
}
