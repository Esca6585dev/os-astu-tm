// Package jobs PostgreSQL-iň içindäki job navbaty we maşyn amallarynyň worker-i.
package jobs

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
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
)

// Payload — job bilen gelýän goşmaça maglumat. Parol bu ýerde saklanmaýar:
// döretmek üçin parol gerek bolsa, ol job tamamlananda pozulýar.
type Payload struct {
	Password string `json:"password,omitempty"`
	ImageID  int64  `json:"image_id,omitempty"`
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
