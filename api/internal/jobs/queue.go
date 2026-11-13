// Package jobs PostgreSQL-iň içindäki job navbaty we maşyn amallarynyň worker-i.
package jobs

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/Esca6585dev/os-astu-tm/api/internal/hv"
)

const (
	ActCreate        = "create"
	ActStart         = "start"
	ActStop          = "stop"
	ActReboot        = "reboot"
	ActReinstall     = "reinstall"
	ActResetPassword = "reset_password"
	ActDelete        = "delete"
	ActSuspend       = "suspend"
	ActUnsuspend     = "unsuspend"
	ActFirewall      = "firewall"
	ActBackup        = "backup"
	ActRestore       = "restore"
	ActBackupDelete  = "backup_delete"
)

// Payload — job bilen gelýän goşmaça maglumat. Parol bu ýerde saklanmaýar:
// döretmek üçin parol gerek bolsa, ol job tamamlananda pozulýar.
type Payload struct {
	Password string `json:"password,omitempty"`
	ImageID  int64  `json:"image_id,omitempty"`
	BackupID int64  `json:"backup_id,omitempty"`
}

// Enqueue tranzaksiýanyň içinde job goşýar.
func Enqueue(ctx context.Context, tx pgx.Tx, serverID int64, action string, p Payload) (int64, error) {
	b, _ := json.Marshal(p)
	var id int64
	err := tx.QueryRow(ctx,
		`INSERT INTO jobs (server_id, action, payload) VALUES ($1,$2,$3) RETURNING id`,
		serverID, action, b).Scan(&id)
	return id, err
}

// Querier — *pgxpool.Pool we pgx.Tx ikisi hem kanagatlandyrýar.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// LoadFirewall maşynyň DB-däki firewall sazlamasyny okaýar.
func LoadFirewall(ctx context.Context, q Querier, serverID int64) (hv.Firewall, error) {
	fw := hv.Firewall{Rules: []hv.FWRule{}}
	if err := q.QueryRow(ctx, `SELECT fw_policy_in FROM servers WHERE id=$1`, serverID).Scan(&fw.PolicyIn); err != nil {
		return fw, err
	}
	rows, err := q.Query(ctx, `SELECT direction, action, proto, dport, addr, comment FROM firewall_rules WHERE server_id=$1 ORDER BY pos, id`, serverID)
	if err != nil {
		return fw, err
	}
	fw.Rules, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (hv.FWRule, error) {
		var x hv.FWRule
		return x, r.Scan(&x.Direction, &x.Action, &x.Proto, &x.Dport, &x.Addr, &x.Comment)
	})
	return fw, err
}

// BackupTag Proxmox-daky bekap faýlyna goşulýan bellik (faýly tapmak üçin).
func BackupTag(id int64) string { return "os.astu.tm bekap #" + strconv.FormatInt(id, 10) }
