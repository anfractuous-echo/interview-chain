package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"interview-chain/internal/payment"
)

type config struct {
	count      int
	outputPath string
}

func parseConfig(args []string) (config, error) {
	var cfg config
	flags := flag.NewFlagSet("payment-generator", flag.ContinueOnError)
	flags.IntVar(&cfg.count, "count", 100000, "number of payments")
	flags.StringVar(&cfg.outputPath, "out", "payments.generated.jsonl", "output path")
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
	if cfg.count < 1 {
		log.Fatal("count must be positive")
	}
	file, err := os.Create(cfg.outputPath)
	if err != nil {
		log.Fatal(err)
	}
	writer := bufio.NewWriter(file)
	for n := 1; n <= cfg.count; n++ {
		from, to := "alice", "bob"
		if n%2 == 0 {
			from, to = "carol", "dave"
		}
		item := payment.Instruction{
			ID:     fmt.Sprintf("load-%06d", n),
			From:   from,
			To:     to,
			Amount: fmt.Sprintf("0.%06d", n%999999+1),
		}
		if err := json.NewEncoder(writer).Encode(item); err != nil {
			log.Fatal(err)
		}
	}
	if err := writer.Flush(); err != nil {
		log.Fatal(err)
	}
	if err := file.Close(); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %d payments to %s", cfg.count, cfg.outputPath)
}
