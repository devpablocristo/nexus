package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"

	"data-plane/cmd/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	db, err := sql.Open("postgres", cfg.DB.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := db.ExecContext(ctx, "delete from idempotency_keys where expires_at < now()")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	n, _ := res.RowsAffected()
	fmt.Printf("idempotency_deleted=%d\n", n)

	res2, err := db.ExecContext(ctx, "update pending_approvals set status = 'expired' where status = 'pending' and expires_at < now()")
	if err != nil {
		fmt.Fprintf(os.Stderr, "approval expire: %s\n", err.Error())
	} else {
		n2, _ := res2.RowsAffected()
		fmt.Printf("approvals_expired=%d\n", n2)
	}
}
