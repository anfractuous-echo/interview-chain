package main

import (
	"testing"
	"time"
)

func TestParseConfigUsesTenSecondBlockIntervalByDefault(t *testing.T) {
	t.Parallel()
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.blockInterval != 10*time.Second {
		t.Fatalf("block interval = %s, want 10s", cfg.blockInterval)
	}
}

func TestParseConfigAcceptsCustomBlockInterval(t *testing.T) {
	t.Parallel()
	cfg, err := parseConfig([]string{"-block-interval", "250ms"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.blockInterval != 250*time.Millisecond {
		t.Fatalf("block interval = %s, want 250ms", cfg.blockInterval)
	}
}

func TestParseConfigRejectsNonPositiveBlockInterval(t *testing.T) {
	t.Parallel()
	if _, err := parseConfig([]string{"-block-interval", "0s"}); err == nil {
		t.Fatal("expected non-positive block interval to be rejected")
	}
}
