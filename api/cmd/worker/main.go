// os.astu.tm worker: Proxmox amallary we sagatlaýyn billing.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Esca6585dev/os-astu-tm/api/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, pool, hyp, err := app.Init(ctx)
	if err != nil {
		slog.Error("start", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	app.RunWorker(ctx, cfg, pool, hyp)
}
