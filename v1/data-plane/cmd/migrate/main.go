package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"data-plane/cmd/config"
)

func main() {
	cmd := flag.String("cmd", "up", "up|down")
	steps := flag.Int("steps", 1, "down steps (default 1)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	db, err := OpenDB(cfg.DB.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	r := Runner{DB: db, Dir: cfg.Migrations.Dir}
	switch *cmd {
	case "up":
		if err := r.Up(ctx); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	case "down":
		if err := r.Down(ctx, *steps); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown cmd:", *cmd)
		os.Exit(1)
	}
}
