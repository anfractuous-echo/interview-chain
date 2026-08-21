package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"interview-chain/internal/toychain"
	"interview-chain/internal/wallet"
)

type config struct {
	listenAddress string
	walletPath    string
	blockInterval time.Duration
}

func parseConfig(args []string) (config, error) {
	var cfg config
	flags := flag.NewFlagSet("toy-chain", flag.ContinueOnError)
	flags.StringVar(&cfg.listenAddress, "listen", ":8080", "HTTP listen address")
	flags.StringVar(&cfg.walletPath, "wallet", "input-data/wallet.json", "wallet input path")
	flags.DurationVar(&cfg.blockInterval, "block-interval", 10*time.Second, "time between blocks")
	if err := flags.Parse(args); err != nil {
		return config{}, err
	}
	if cfg.blockInterval <= 0 {
		return config{}, fmt.Errorf("block interval must be positive")
	}
	return cfg, nil
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	walletData, err := wallet.Load(cfg.walletPath)
	if err != nil {
		log.Fatal(err)
	}
	ledger := toychain.NewLedger(walletData)
	server := &http.Server{
		Addr:              cfg.listenAddress,
		Handler:           toychain.NewServer(ledger).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		ticker := time.NewTicker(cfg.blockInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				block := ledger.MineBlock()
				log.Printf("mined block height=%d transactions=%d", block.Height, len(block.Transactions))
			}
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("toy blockchain listening on %s, block interval %s", cfg.listenAddress, cfg.blockInterval)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
