// os.astu.tm HTTP API.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Esca6585dev/os-astu-tm/api/internal/app"
	"github.com/Esca6585dev/os-astu-tm/api/internal/httpapi"
	"github.com/Esca6585dev/os-astu-tm/api/internal/mail"
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

	api := &httpapi.API{Cfg: cfg, DB: pool, HV: hyp, Mail: mail.New(cfg)}
	if err := api.EnsureAdmin(ctx); err != nil {
		slog.Error("admin", "err", err)
		os.Exit(1)
	}
	// RUN_WORKER=true: worker-i şu prosesiň içinde işletmek (HYPERVISOR=fake bilen synag üçin hökmany)
	if os.Getenv("RUN_WORKER") == "true" {
		go app.RunWorker(ctx, cfg, pool, hyp)
	}
	srv := api.App()
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown()
	}()
	slog.Info("API işleýär", "addr", cfg.HTTPAddr, "hypervisor", cfg.Hypervisor)
	if err := srv.Listen(cfg.HTTPAddr); err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}
}
