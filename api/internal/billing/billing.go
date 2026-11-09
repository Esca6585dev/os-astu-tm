// Package billing sagatlaýyn tutum, aýlyk çäk, saklamak (suspend) we pozmak.
//
// Düzgünler:
//   - Ähli mukdarlar teňňede (int64). 1 aý = 720 sagat.
//   - Her sagat: charge = min(price, round(price*n/720)) − şu aý eýýäm tutulan,
//     bu ýerde n — şu aýda hasaplanan sagatlaryň sany (şu sagat bilen).
//   - server_usage (server_id, hour) PRIMARY KEY bir sagadyň iki gezek tutulmagynyň öňüni alýar.
package billing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Esca6585dev/os-astu-tm/api/internal/config"
	"github.com/Esca6585dev/os-astu-tm/api/internal/jobs"
	"github.com/Esca6585dev/os-astu-tm/api/internal/mail"
)

const HoursPerMonth = 720

// HourlyCharge n-nji sagat üçin tutulmaly mukdary hasaplaýar.
func HourlyCharge(priceMonthly int64, hoursBilledBefore int64, chargedBefore int64) int64 {
	n := hoursBilledBefore + 1
	target := (priceMonthly*n*2 + HoursPerMonth) / (2 * HoursPerMonth) // round half up
	if target > priceMonthly {
		target = priceMonthly
	}
	c := target - chargedBefore
	if c < 0 {
		return 0
	}
	return c
}

type Billing struct {
	DB   *pgxpool.Pool
	Mail *mail.Mailer
	Cfg  config.Config
	Loc  *time.Location
}

func (b *Billing) loc() *time.Location {
	if b.Loc != nil {
		return b.Loc
	}
	return time.Local
}

// Tick soňky tamamlanan sagatlary hasaplaýar (idempotent) we balans düzgünlerini ýerine ýetirýär.
func (b *Billing) Tick(ctx context.Context, now time.Time) {
	cur := now.In(b.loc()).Truncate(time.Hour)
	for i := 3; i >= 1; i-- {
		if err := b.ChargeHour(ctx, cur.Add(-time.Duration(i)*time.Hour)); err != nil {
			slog.Error("billing hour", "err", err)
		}
	}
	if err := b.Enforce(ctx); err != nil {
		slog.Error("billing enforce", "err", err)
	}
}

// ChargeHour [hour, hour+1h) aralygy üçin ähli tölegli maşynlardan pul tutýar.
func (b *Billing) ChargeHour(ctx context.Context, hour time.Time) error {
	end := hour.Add(time.Hour)
	rows, err := b.DB.Query(ctx, `
		SELECT s.id FROM servers s
		WHERE s.created_at < $2
		  AND (s.deleted_at IS NULL OR s.deleted_at > $1)
		  AND (s.status NOT IN ('error','suspended','deleted') OR s.deleted_at > $1)
		  AND NOT EXISTS (SELECT 1 FROM server_usage u WHERE u.server_id=s.id AND u.hour=$1)`, hour, end)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := b.chargeServer(ctx, id, hour); err != nil {
			slog.Error("charge server", "server", id, "hour", hour, "err", err)
		}
	}
	return nil
}

func (b *Billing) chargeServer(ctx context.Context, serverID int64, hour time.Time) error {
	monthStart := time.Date(hour.Year(), hour.Month(), 1, 0, 0, 0, 0, hour.Location())
	tx, err := b.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var userID, price int64
	var hostname string
	if err := tx.QueryRow(ctx, `SELECT s.user_id, p.price_monthly, s.hostname FROM servers s JOIN plans p ON p.id=s.plan_id WHERE s.id=$1`,
		serverID).Scan(&userID, &price, &hostname); err != nil {
		return err
	}
	var n, charged int64
	if err := tx.QueryRow(ctx, `SELECT count(*), COALESCE(sum(amount),0) FROM server_usage WHERE server_id=$1 AND hour >= $2 AND hour < $3`,
		serverID, monthStart, hour).Scan(&n, &charged); err != nil {
		return err
	}
	amount := HourlyCharge(price, n, charged)

	tag, err := tx.Exec(ctx, `INSERT INTO server_usage (server_id, hour, amount) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, serverID, hour, amount)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil // başga proses eýýäm hasaplapdyr
	}
	if amount > 0 {
		var balance int64
		if err := tx.QueryRow(ctx, `UPDATE users SET balance=balance-$2 WHERE id=$1 RETURNING balance`, userID, amount).Scan(&balance); err != nil {
			return err
		}
		note := fmt.Sprintf("%s · %s–%s", hostname, hour.Format("15:04"), hour.Add(time.Hour).Format("15:04"))
		var txID int64
		if err := tx.QueryRow(ctx, `INSERT INTO transactions (user_id, type, amount, balance_after, server_id, note, created_at)
			VALUES ($1,'charge',$2,$3,$4,$5,$6) RETURNING id`, userID, -amount, balance, serverID, note, hour.Add(time.Hour)).Scan(&txID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE server_usage SET transaction_id=$3 WHERE server_id=$1 AND hour=$2`, serverID, hour, txID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Enforce: balansy gutaranlary saklamak, doldurylanlary başlatmak, köne saklananlary pozmak, duýduryşlar.
func (b *Billing) Enforce(ctx context.Context) error {
	// 1) Balans < 0 → saklamak
	if err := b.enqueueFor(ctx, `
		SELECT s.id FROM servers s JOIN users u ON u.id=s.user_id
		WHERE u.balance < 0 AND s.status IN ('running','stopped') AND NOT s.busy AND s.deleted_at IS NULL`, jobs.ActSuspend); err != nil {
		return err
	}
	// 2) Balans > 0 → saklanan maşynlary başlatmak
	if err := b.enqueueFor(ctx, `
		SELECT s.id FROM servers s JOIN users u ON u.id=s.user_id
		WHERE u.balance > 0 AND u.status='active' AND s.status='suspended' AND NOT s.busy`, jobs.ActUnsuspend); err != nil {
		return err
	}
	// 3) Uzak wagt saklanan → pozmak
	deleteBefore := time.Now().Add(-b.Cfg.SuspendDeleteAfter)
	if err := b.enqueueFor(ctx, `
		SELECT id FROM servers WHERE status='suspended' AND NOT busy AND suspended_at < $1`, jobs.ActDelete, deleteBefore); err != nil {
		return err
	}
	// 4) Pozmazdan 2 gün öň duýduryş
	b.warnDeletes(ctx, deleteBefore.Add(48*time.Hour))
	// 5) Az balans duýduryşy
	b.warnLowBalance(ctx)
	return nil
}

func (b *Billing) enqueueFor(ctx context.Context, query string, action string, args ...any) error {
	rows, err := b.DB.Query(ctx, query, args...)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return err
	}
	for _, id := range ids {
		tx, err := b.DB.Begin(ctx)
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE servers SET busy=true WHERE id=$1 AND NOT busy`, id)
		if err == nil && tag.RowsAffected() == 1 {
			if _, err = jobs.Enqueue(ctx, tx, id, action, jobs.Payload{}); err == nil {
				err = tx.Commit(ctx)
				slog.Info("billing enqueue", "server", id, "action", action)
			}
		}
		tx.Rollback(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Billing) warnDeletes(ctx context.Context, warnBefore time.Time) {
	rows, err := b.DB.Query(ctx, `
		UPDATE servers s SET delete_warned_at=now() FROM users u
		WHERE u.id=s.user_id AND s.status='suspended' AND s.delete_warned_at IS NULL AND s.suspended_at < $1
		RETURNING u.email, s.hostname`, warnBefore)
	if err != nil {
		slog.Error("warn deletes", "err", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var email, host string
		if rows.Scan(&email, &host) == nil {
			_ = b.Mail.Send(email, "Maşynyňyz 2 günden pozular: "+host,
				fmt.Sprintf("Balansyňyz gutaranlygy sebäpli %s maşyny saklanýar.\nBalansy 2 günüň dowamynda doldurmasaňyz, maşyn we onuň diski pozular.\n\n%s/billing", host, b.Cfg.PublicURL))
		}
	}
}

func (b *Billing) warnLowBalance(ctx context.Context) {
	rows, err := b.DB.Query(ctx, `
		WITH rate AS (
			SELECT s.user_id, sum(p.price_monthly)::bigint AS monthly
			FROM servers s JOIN plans p ON p.id=s.plan_id
			WHERE s.status IN ('running','stopped') AND s.deleted_at IS NULL GROUP BY s.user_id)
		UPDATE users u SET low_balance_notified_at=now() FROM rate r
		WHERE r.user_id=u.id AND u.balance >= 0 AND u.balance < r.monthly*24/720
		  AND (u.low_balance_notified_at IS NULL OR u.low_balance_notified_at < now() - interval '24 hours')
		RETURNING u.email, u.balance`)
	if err != nil {
		slog.Error("warn low balance", "err", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var email string
		var bal int64
		if rows.Scan(&email, &bal) == nil {
			_ = b.Mail.Send(email, "Balansyňyz az galdy",
				fmt.Sprintf("Balansyňyz %s TMT. Ol 24 sagatdan az ýeter.\nBalans gutarsa, maşynlaryňyz durdurylar.\n\n%s/billing", FormatTMT(bal), b.Cfg.PublicURL))
		}
	}
}

func FormatTMT(tenge int64) string {
	sign := ""
	if tenge < 0 {
		sign, tenge = "-", -tenge
	}
	return fmt.Sprintf("%s%d.%02d", sign, tenge/100, tenge%100)
}
