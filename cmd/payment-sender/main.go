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
	payments, err := database.ListReceivedPayments(ctx)
	if err != nil {
		log.Fatal(err)
	}
	processor := sender.NewProcessor(database, sender.NewNodeClient(cfg.nodeURL), wallet.SigningKeys(walletData))
	for _, payment := range payments {
		if err := processor.ProcessPayment(ctx, payment); err != nil {
			log.Fatal(fmt.Errorf("process payment %s: %w", payment.ID, err))
		}
	}
	log.Printf("processed %d payments", len(payments))
}
