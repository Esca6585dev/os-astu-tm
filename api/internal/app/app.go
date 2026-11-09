// Package app ähli proseslere umumy başlangyç: log, DB, hypervisor.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Esca6585dev/os-astu-tm/api/internal/billing"
	"github.com/Esca6585dev/os-astu-tm/api/internal/config"
	"github.com/Esca6585dev/os-astu-tm/api/internal/db"
	"github.com/Esca6585dev/os-astu-tm/api/internal/hv"
	"github.com/Esca6585dev/os-astu-tm/api/internal/jobs"
	"github.com/Esca6585dev/os-astu-tm/api/internal/mail"
)

func Init(ctx context.Context) (config.Config, *pgxpool.Pool, hv.Hypervisor, error) {
	cfg := config.Load()
	var h slog.Handler = slog.NewTextHandler(os.Stdout, nil)
	if cfg.Env == "prod" {
		h = slog.NewJSONHandler(os.Stdout, nil)
	}
	slog.SetDefault(slog.New(h))

	var pool *pgxpool.Pool
	var err error
	for i := 0; i < 30; i++ { // docker-compose-da postgres-e garaşmak
		if pool, err = db.Connect(ctx, cfg.DatabaseURL); err == nil {
			break
		}
		slog.Warn("postgres garaşylýar", "err", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return cfg, nil, nil, err
	}
	if err := db.Migrate(ctx, pool); err != nil {
		return cfg, nil, nil, err
	}

	var hyp hv.Hypervisor
	switch cfg.Hypervisor {
	case "proxmox":
		hyp, err = hv.NewProxmox(hv.ProxmoxOptions{URL: cfg.PVEURL, TokenID: cfg.PVETokenID, TokenSecret: cfg.PVETokenSecret,
			InsecureTLS: cfg.PVEInsecureTLS, CAFile: cfg.PVECAFile, Disk: cfg.PVEDisk, Firewall: cfg.PVEFirewall})
		if err != nil {
			return cfg, nil, nil, err
		}
	case "fake":
		slog.Warn("HYPERVISOR=fake: hakyky Proxmox ulanylmaýar (diňe synag üçin)")
		hyp = hv.NewFake()
	default:
		return cfg, nil, nil, fmt.Errorf("HYPERVISOR nädogry: %q (proxmox | fake)", cfg.Hypervisor)
	}
	return cfg, pool, hyp, nil
}

// RunWorker job worker-ini we sagatlaýyn billing-i işledýär (ctx ýatyrylýança bloklaýar).
func RunWorker(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, hyp hv.Hypervisor) {
	b := &billing.Billing{DB: pool, Mail: mail.New(cfg), Cfg: cfg}
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		b.Tick(ctx, time.Now())
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				b.Tick(ctx, now)
			}
		}
	}()
	slog.Info("worker işleýär", "hypervisor", cfg.Hypervisor)
	(&jobs.Worker{DB: pool, HV: hyp, Concurrency: 4}).Run(ctx)
}
