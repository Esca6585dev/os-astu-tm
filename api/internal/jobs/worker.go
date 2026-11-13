package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Esca6585dev/os-astu-tm/api/internal/hv"
)

type Worker struct {
	DB          *pgxpool.Pool
	HV          hv.Hypervisor
	Concurrency int
}

type job struct {
	ID       int64
	ServerID int64
	Action   string
	Payload  Payload
}

// Run job navbatyny ctx ýatyrylýança işleýär.
func (w *Worker) Run(ctx context.Context) {
	n := w.Concurrency
	if n <= 0 {
		n = 4
	}
	// Worker täzeden başlanda "running" galan joblary yzyna navbata goýýarys
	if _, err := w.DB.Exec(ctx, `UPDATE jobs SET status='queued' WHERE status='running'`); err != nil {
		slog.Error("requeue", "err", err)
	}
	sem := make(chan struct{}, n)
	for ctx.Err() == nil {
		j, err := w.claim(ctx)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) && ctx.Err() == nil {
				slog.Error("claim job", "err", err)
			}
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
			continue
		}
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			w.process(ctx, j)
		}()
	}
}

func (w *Worker) claim(ctx context.Context) (job, error) {
	var j job
	var raw []byte
	err := w.DB.QueryRow(ctx, `
		UPDATE jobs SET status='running', attempts=attempts+1
		WHERE id = (
			SELECT id FROM jobs j
			WHERE status='queued' AND run_after <= now()
			  -- bir maşynda bir wagtda diňe bir amal
			  AND NOT EXISTS (SELECT 1 FROM jobs r WHERE r.server_id=j.server_id AND r.status='running')
			ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED)
		RETURNING id, server_id, action, payload`).Scan(&j.ID, &j.ServerID, &j.Action, &raw)
	if err != nil {
		return j, err
	}
	_ = json.Unmarshal(raw, &j.Payload)
	return j, nil
}

func (w *Worker) process(ctx context.Context, j job) {
	timeout := 30 * time.Minute
	if j.Action == ActBackup || j.Action == ActRestore {
		timeout = 4 * time.Hour // uly diskleriň bekaby uzak dowam edýär
	}
	jctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	log := slog.With("job", j.ID, "server", j.ServerID, "action", j.Action)
	log.Info("job start")
	err := w.handle(jctx, j)
	status, msg := "done", ""
	if err != nil {
		status, msg = "failed", err.Error()
		log.Error("job failed", "err", err)
	} else {
		log.Info("job done")
	}
	// Paroly job-dan pozýarys
	_, dbErr := w.DB.Exec(context.Background(),
		`UPDATE jobs SET status=$2, error=$3, finished_at=now(), payload='{}' WHERE id=$1`, j.ID, status, msg)
	if dbErr != nil {
		log.Error("job finish", "err", dbErr)
	}
	_, _ = w.DB.Exec(context.Background(), `UPDATE servers SET busy=false WHERE id=$1`, j.ServerID)
}

type srv struct {
	ID, UserID   int64
	Hostname     string
	VMID         *int
	Status       string
	Node         string
	Storage      string
	Bridge       string
	Cores, RAM   int
	Disk, BW     int
	TemplateVMID int
	IP           string
	Prefix       int
	Gateway, DNS string
}

func (w *Worker) load(ctx context.Context, id int64, imageOverride int64) (srv, error) {
	var s srv
	err := w.DB.QueryRow(ctx, `
		SELECT s.id, s.user_id, s.hostname, s.proxmox_vmid, s.status,
		       n.name, n.storage, n.bridge,
		       p.vcpu, p.ram_mb, p.disk_gb, p.bandwidth_mbit,
		       i.proxmox_template_vmid,
		       COALESCE(a.address,''), COALESCE(a.prefix,24), COALESCE(ip.gateway,''), COALESCE(ip.dns,'')
		FROM servers s
		JOIN nodes n ON n.id=s.node_id
		JOIN plans p ON p.id=s.plan_id
		JOIN images i ON i.id = COALESCE(NULLIF($2::bigint,0), s.image_id)
		LEFT JOIN ip_addresses a ON a.server_id=s.id
		LEFT JOIN ip_pools ip ON ip.id=a.pool_id
		WHERE s.id=$1`, id, imageOverride).Scan(
		&s.ID, &s.UserID, &s.Hostname, &s.VMID, &s.Status,
		&s.Node, &s.Storage, &s.Bridge,
		&s.Cores, &s.RAM, &s.Disk, &s.BW, &s.TemplateVMID,
		&s.IP, &s.Prefix, &s.Gateway, &s.DNS)
	return s, err
}

func (w *Worker) sshKeys(ctx context.Context, serverID int64) ([]string, error) {
	rows, err := w.DB.Query(ctx, `SELECT k.public_key FROM server_ssh_keys sk JOIN ssh_keys k ON k.id=sk.ssh_key_id WHERE sk.server_id=$1`, serverID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (w *Worker) setStatus(ctx context.Context, id int64, status string, extra string) error {
	_, err := w.DB.Exec(ctx, `UPDATE servers SET status=$2`+extra+` WHERE id=$1`, id, status)
	return err
}

func (w *Worker) spec(ctx context.Context, s srv, vmid int, password string) (hv.VMSpec, error) {
	keys, err := w.sshKeys(ctx, s.ID)
	if err != nil {
		return hv.VMSpec{}, err
	}
	if s.IP == "" {
		return hv.VMSpec{}, errors.New("maşyna IP berilmedik")
	}
	fw, err := LoadFirewall(ctx, w.DB, s.ID)
	if err != nil {
		return hv.VMSpec{}, err
	}
	return hv.VMSpec{
		Node: s.Node, VMID: vmid, Name: s.Hostname, TemplateVMID: s.TemplateVMID,
		Storage: s.Storage, Bridge: s.Bridge, Cores: s.Cores, MemoryMB: s.RAM, DiskGB: s.Disk,
		BandwidthMbit: s.BW, IP: s.IP, Prefix: s.Prefix, Gateway: s.Gateway, DNS: s.DNS,
		Password: password, SSHKeys: keys, Firewall: fw,
	}, nil
}

type backupRow struct {
	Volid   *string
	Storage string
	ImageID int64
}

func (w *Worker) backup(ctx context.Context, id, serverID int64) (backupRow, error) {
	var b backupRow
	err := w.DB.QueryRow(ctx, `SELECT volid, storage, image_id FROM backups WHERE id=$1 AND server_id=$2`, id, serverID).
		Scan(&b.Volid, &b.Storage, &b.ImageID)
	if err != nil {
		return b, fmt.Errorf("bekap #%d: %w", id, err)
	}
	return b, nil
}

func (w *Worker) backupFailed(id int64, cause error) error {
	_, _ = w.DB.Exec(context.Background(), `UPDATE backups SET status='error', error=$2, finished_at=now() WHERE id=$1`,
		id, ShortError(cause.Error()))
	return cause
}

// deleteBackups pozulýan maşynyň ähli bekap faýllaryny pozýar. Şowsuz bolsa, bekap
// "error" ýagdaýynda galýar (admin görer), maşyny pozmak bolsa dowam edýär.
func (w *Worker) deleteBackups(ctx context.Context, s srv) {
	rows, err := w.DB.Query(ctx, `SELECT id, volid FROM backups WHERE server_id=$1 AND status <> 'deleted'`, s.ID)
	if err != nil {
		slog.Error("backups list", "server", s.ID, "err", err)
		return
	}
	type b struct {
		ID    int64
		Volid *string
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (b, error) {
		var x b
		return x, r.Scan(&x.ID, &x.Volid)
	})
	if err != nil {
		slog.Error("backups list", "server", s.ID, "err", err)
		return
	}
	for _, x := range list {
		if x.Volid != nil {
			if err := w.HV.DeleteBackup(ctx, s.Node, *x.Volid); err != nil {
				slog.Error("backup delete", "backup", x.ID, "err", err)
				_ = w.backupFailed(x.ID, err)
				continue
			}
		}
		_, _ = w.DB.Exec(ctx, `UPDATE backups SET status='deleted', finished_at=now() WHERE id=$1`, x.ID)
	}
}

func (w *Worker) handle(ctx context.Context, j job) error {
	s, err := w.load(ctx, j.ServerID, j.Payload.ImageID)
	if err != nil {
		return fmt.Errorf("server load: %w", err)
	}
	if s.VMID == nil && j.Action != ActCreate && j.Action != ActDelete && j.Action != ActBackupDelete {
		return errors.New("maşynyň VMID-i ýok")
	}

	switch j.Action {
	case ActCreate:
		vmid, err := w.HV.NextVMID(ctx)
		if err != nil {
			return w.fail(ctx, s, err)
		}
		if _, err := w.DB.Exec(ctx, `UPDATE servers SET proxmox_vmid=$2 WHERE id=$1`, s.ID, vmid); err != nil {
			return w.fail(ctx, s, err)
		}
		spec, err := w.spec(ctx, s, vmid, j.Payload.Password)
		if err != nil {
			return w.fail(ctx, s, err)
		}
		if err := w.HV.Create(ctx, spec); err != nil {
			_ = w.HV.Destroy(context.Background(), s.Node, vmid)
			return w.fail(ctx, s, err)
		}
		return w.setStatus(ctx, s.ID, "running", "")

	case ActStart, ActUnsuspend:
		if err := w.HV.Start(ctx, s.Node, *s.VMID); err != nil {
			return err
		}
		return w.setStatus(ctx, s.ID, "running", ", suspended_at=NULL")

	case ActStop:
		if err := w.HV.Stop(ctx, s.Node, *s.VMID); err != nil {
			return err
		}
		return w.setStatus(ctx, s.ID, "stopped", "")

	case ActSuspend:
		if err := w.HV.Stop(ctx, s.Node, *s.VMID); err != nil {
			return err
		}
		return w.setStatus(ctx, s.ID, "suspended", ", suspended_at=now()")

	case ActReboot:
		return w.HV.Reboot(ctx, s.Node, *s.VMID)

	case ActResetPassword:
		return w.HV.SetPassword(ctx, s.Node, *s.VMID, "root", j.Payload.Password)

	case ActReinstall:
		vmid := *s.VMID
		if err := w.HV.Destroy(ctx, s.Node, vmid); err != nil {
			return w.fail(ctx, s, fmt.Errorf("destroy: %w", err))
		}
		spec, err := w.spec(ctx, s, vmid, j.Payload.Password)
		if err != nil {
			return w.fail(ctx, s, err)
		}
		if err := w.HV.Create(ctx, spec); err != nil {
			return w.fail(ctx, s, err)
		}
		if j.Payload.ImageID != 0 {
			if _, err := w.DB.Exec(ctx, `UPDATE servers SET image_id=$2 WHERE id=$1`, s.ID, j.Payload.ImageID); err != nil {
				return err
			}
		}
		return w.setStatus(ctx, s.ID, "running", "")

	case ActFirewall:
		fw, err := LoadFirewall(ctx, w.DB, s.ID)
		if err != nil {
			return err
		}
		return w.HV.SetFirewall(ctx, s.Node, *s.VMID, s.IP, fw)

	case ActBackup:
		id := j.Payload.BackupID
		b, err := w.backup(ctx, id, s.ID)
		if err != nil {
			return w.backupFailed(id, err)
		}
		info, err := w.HV.Backup(ctx, s.Node, *s.VMID, b.Storage, BackupTag(id))
		if err != nil {
			return w.backupFailed(id, err)
		}
		_, err = w.DB.Exec(ctx, `UPDATE backups SET status='ready', volid=$2, size_bytes=$3, finished_at=now(), error='' WHERE id=$1`,
			id, info.Volid, info.Size)
		return err

	case ActRestore:
		id := j.Payload.BackupID
		b, err := w.backup(ctx, id, s.ID)
		if err == nil && b.Volid == nil {
			err = errors.New("bekap faýly ýok")
		}
		if err != nil {
			_, _ = w.DB.Exec(context.Background(), `UPDATE backups SET status='ready' WHERE id=$1 AND status='restoring'`, id)
			_ = w.setStatus(context.Background(), s.ID, w.actualStatus(s), "")
			return err
		}
		// Ýalňyşlyk bolsa-da bekap ulanmaga taýýar galýar
		defer func() {
			_, _ = w.DB.Exec(context.Background(), `UPDATE backups SET status='ready' WHERE id=$1 AND status='restoring'`, id)
		}()
		if err := w.HV.RestoreBackup(ctx, s.Node, *s.VMID, *b.Volid, s.Storage); err != nil {
			return w.fail(ctx, s, err)
		}
		// Bekapdaky firewall köne bolup biler: häzirki düzgünleri gaýtadan goýýarys
		fw, err := LoadFirewall(ctx, w.DB, s.ID)
		if err == nil {
			err = w.HV.SetFirewall(ctx, s.Node, *s.VMID, s.IP, fw)
		}
		if err != nil {
			return w.fail(ctx, s, fmt.Errorf("firewall: %w", err))
		}
		if _, err := w.DB.Exec(ctx, `UPDATE servers SET image_id=$2 WHERE id=$1`, s.ID, b.ImageID); err != nil {
			return err
		}
		if err := w.HV.Start(ctx, s.Node, *s.VMID); err != nil {
			_ = w.setStatus(context.Background(), s.ID, "stopped", "")
			return fmt.Errorf("dikeldildi, ýöne başlamady: %w", err)
		}
		return w.setStatus(ctx, s.ID, "running", "")

	case ActBackupDelete:
		id := j.Payload.BackupID
		b, err := w.backup(ctx, id, s.ID)
		if err != nil {
			return w.backupFailed(id, err)
		}
		if b.Volid != nil {
			if err := w.HV.DeleteBackup(ctx, s.Node, *b.Volid); err != nil {
				return w.backupFailed(id, err)
			}
		}
		_, err = w.DB.Exec(ctx, `UPDATE backups SET status='deleted', finished_at=now() WHERE id=$1`, id)
		return err

	case ActDelete:
		if s.VMID != nil {
			if err := w.HV.Destroy(ctx, s.Node, *s.VMID); err != nil {
				return err
			}
		}
		w.deleteBackups(ctx, s)
		tx, err := w.DB.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `UPDATE ip_addresses SET server_id=NULL WHERE server_id=$1`, s.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE servers SET status='deleted', deleted_at=COALESCE(deleted_at, now()), proxmox_vmid=NULL WHERE id=$1`, s.ID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	return fmt.Errorf("näbelli amal: %s", j.Action)
}

// actualStatus maşynyň hypervisor-daky hakyky ýagdaýyny (running | stopped) gaýtarýar.
func (w *Worker) actualStatus(s srv) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if u, err := w.HV.Usage(ctx, s.Node, *s.VMID); err == nil && u.Status == "running" {
		return "running"
	}
	return "stopped"
}

// fail döretmek ýa-da täzeden gurmak şowsuz bolanda maşyny "error" ýagdaýyna geçirýär.
func (w *Worker) fail(ctx context.Context, s srv, cause error) error {
	_ = w.setStatus(context.Background(), s.ID, "error", "")
	return cause
}

// Hata habarynyň ulanyja görkezilýän gysga görnüşi
func ShortError(msg string) string {
	if i := strings.Index(msg, "\n"); i > 0 {
		msg = msg[:i]
	}
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	return msg
}
