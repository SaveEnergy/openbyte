package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	cfg := parseFlags()
	if err := validateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "loadtest: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.duration)
	defer cancel()

	bytesSent, bytesRecv, workerErrs := runLoadtest(ctx, cfg)

	seconds := cfg.duration.Seconds()
	if seconds <= 0 {
		seconds = 1
	}
	fmt.Printf("mode=%s concurrency=%d duration=%s sent_bytes=%d recv_bytes=%d sent_mbps=%.2f recv_mbps=%.2f\n",
		cfg.mode,
		cfg.concurrency,
		cfg.duration,
		bytesSent,
		bytesRecv,
		float64(bytesSent*8)/seconds/1_000_000,
		float64(bytesRecv*8)/seconds/1_000_000,
	)
	if workerErrs > 0 {
		fmt.Fprintf(os.Stderr, "loadtest: %d worker(s) failed\n", workerErrs)
		os.Exit(1)
	}
}
