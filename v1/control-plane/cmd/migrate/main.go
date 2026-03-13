package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"gorm.io/gorm"

	"control-plane/cmd/config"
	gormdb "nexus/pkg/databases/sql/gorm"
	"nexus/pkg/utils"
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

	db, err := gormdb.Open(gormdb.OpenOptions{DatabaseURL: cfg.DB.DatabaseURL}, &gorm.Config{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer sqlDB.Close()

	m := utils.Migrator{DB: sqlDB, Dir: cfg.Migrations.Dir}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	switch *cmd {
	case "up":
		err = m.Up(ctx)
	case "down":
		err = m.Down(ctx, *steps)
	default:
		err = fmt.Errorf("unknown cmd: %s", *cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
