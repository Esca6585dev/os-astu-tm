package httpapi

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"github.com/Esca6585dev/os-astu-tm/api/internal/hv"
	"github.com/Esca6585dev/os-astu-tm/api/internal/jobs"
)

var errSuspended = E(402, "suspended", "Maşyn balans gutaranlygy sebäpli öçürildi. Balansy dolduryň.")

const maxFWRules = 50

// cleanText dolandyryş simwollaryny aýyrýar, boşluklary jebisleýär we uzynlygy çäklendirýär.
func cleanText(s string, maxBytes int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return truncate(strings.Join(strings.Fields(s), " "), maxBytes)
}

// ---------- IP salgylar ----------

type IPAddr struct {
	Address string   `json:"address"`
	Prefix  int      `json:"prefix"`
	Netmask string   `json:"netmask"`
	Gateway string   `json:"gateway"`
	DNS     []string `json:"dns"`
	Server  struct {
		ID       int64  `json:"id"`
		Hostname string `json:"hostname"`
		Status   string `json:"status"`
	} `json:"server"`
	Firewall struct {
		PolicyIn string `json:"policy_in"`
		Rules    int    `json:"rules"`
	} `json:"firewall"`
}

func (a *API) listIPs(c *fiber.Ctx) error {
	rows, err := a.DB.Query(c.Context(), `
		SELECT a.address, a.prefix, ip.gateway, ip.dns, s.id, s.hostname, s.status, s.fw_policy_in,
		       (SELECT count(*) FROM firewall_rules f WHERE f.server_id=s.id)
		FROM ip_addresses a
		JOIN ip_pools ip ON ip.id=a.pool_id
		JOIN servers s ON s.id=a.server_id
		WHERE s.user_id=$1 AND s.deleted_at IS NULL
		ORDER BY s.created_at DESC, a.id`, userOf(c).ID)
	if err != nil {
		return err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (IPAddr, error) {
		var x IPAddr
		var dns string
		err := r.Scan(&x.Address, &x.Prefix, &x.Gateway, &dns, &x.Server.ID, &x.Server.Hostname, &x.Server.Status,
			&x.Firewall.PolicyIn, &x.Firewall.Rules)
		x.Netmask = net.IP(net.CIDRMask(x.Prefix, 32)).String()
		x.DNS = strings.Fields(dns)
		return x, err
	})
	if err != nil {
		return err
	}
	return c.JSON(list)
}

// ---------- Firewall ----------

var portRe = regexp.MustCompile(`^(\d{1,5})(?::(\d{1,5}))?$`)

// cleanRule düzgüni barlaýar we kadaly görnüşe getirýär.
func cleanRule(r hv.FWRule) (hv.FWRule, error) {
	r.Direction = strings.ToLower(trim(r.Direction))
	if r.Direction != "in" && r.Direction != "out" {
		return r, errors.New("ugur nädogry (giriş ýa-da çykyş)")
	}
	r.Action = strings.ToUpper(trim(r.Action))
	if r.Action != "ACCEPT" && r.Action != "DROP" {
		return r, errors.New("hereket nädogry (rugsat ýa-da blok)")
	}
	r.Proto = strings.ToLower(trim(r.Proto))
	switch r.Proto {
	case "", "tcp", "udp", "icmp":
	default:
		return r, errors.New("protokol nädogry (TCP, UDP, ICMP ýa-da hemmesi)")
	}
	r.Dport = strings.ReplaceAll(trim(r.Dport), " ", "")
	if r.Dport != "" {
		if r.Proto != "tcp" && r.Proto != "udp" {
			return r, errors.New("port diňe TCP ýa-da UDP üçin görkezilýär")
		}
		parts := strings.Split(r.Dport, ",")
		if len(parts) > 10 {
			return r, errors.New("bir düzgünde iň köp 10 port ýa-da aralyk")
		}
		for _, p := range parts {
			m := portRe.FindStringSubmatch(p)
			if m == nil {
				return r, fmt.Errorf("port nädogry: %q (mysal: 22, 80,443 ýa-da 8000:8100)", p)
			}
			lo, _ := strconv.Atoi(m[1])
			hi := lo
			if m[2] != "" {
				hi, _ = strconv.Atoi(m[2])
			}
			if lo < 1 || hi > 65535 || lo > hi {
				return r, fmt.Errorf("port 1–65535 aralygynda bolmaly: %q", p)
			}
		}
	}
	r.Addr = trim(r.Addr)
	if r.Addr != "" {
		bad := fmt.Errorf("salgy nädogry: %q (mysal: 10.0.0.5 ýa-da 192.168.0.0/24)", r.Addr)
		if strings.Contains(r.Addr, "/") {
			p, err := netip.ParsePrefix(r.Addr)
			if err != nil {
				return r, bad
			}
			r.Addr = p.Masked().String()
		} else {
			ad, err := netip.ParseAddr(r.Addr)
			if err != nil || ad.Zone() != "" {
				return r, bad
			}
			r.Addr = ad.String()
		}
	}
	r.Comment = cleanText(r.Comment, 60)
	return r, nil
}

func (a *API) getFirewall(c *fiber.Ctx) error {
	s, err := a.ownServer(c)
	if err != nil {
		return err
	}
	fw, err := jobs.LoadFirewall(c.Context(), a.DB, s.ID)
	if err != nil {
		return err
	}
	system := []hv.FWRule{}
	if a.Cfg.PVEFirewall {
		system = append(system, hv.SMTPBlock)
	}
	return c.JSON(fiber.Map{
		"server_id": s.ID, "hostname": s.Hostname, "ip": s.IP, "status": s.Status, "busy": s.Busy,
		"editable":  s.Status == "running" || s.Status == "stopped",
		"policy_in": fw.PolicyIn, "rules": fw.Rules, "system_rules": system, "last_job_error": s.LastJobError,
	})
}

// putFirewall ähli düzgünleri bir gezekde çalyşýar we Proxmox-a ýazmak üçin job goşýar.
func (a *API) putFirewall(c *fiber.Ctx) error {
	s, err := a.ownServer(c)
	if err != nil {
		return err
	}
	if s.Status == "suspended" {
		return errSuspended
	}
	if s.Status != "running" && s.Status != "stopped" {
		return E(409, "bad_state", "Firewall-y diňe işleýän ýa-da durdurylan maşynda üýtgedip bolýar")
	}
	var in struct {
		PolicyIn string      `json:"policy_in"`
		Rules    []hv.FWRule `json:"rules"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	in.PolicyIn = strings.ToUpper(trim(in.PolicyIn))
	if in.PolicyIn == "" {
		in.PolicyIn = "ACCEPT"
	}
	if in.PolicyIn != "ACCEPT" && in.PolicyIn != "DROP" {
		return E(422, "invalid_policy", "Gelýän trafik syýasaty nädogry")
	}
	if len(in.Rules) > maxFWRules {
		return E(422, "too_many_rules", fmt.Sprintf("Iň köp %d düzgün goşup bolýar", maxFWRules))
	}
	for i := range in.Rules {
		r, err := cleanRule(in.Rules[i])
		if err != nil {
			return E(422, "invalid_rule", fmt.Sprintf("%d-nji düzgün: %s", i+1, err))
		}
		in.Rules[i] = r
	}

	ctx := c.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE servers SET busy=true, fw_policy_in=$2 WHERE id=$1 AND NOT busy AND status IN ('running','stopped')`, s.ID, in.PolicyIn)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return E(409, "busy", "Maşynda başga amal ýerine ýetirilýär. Biraz garaşyň.")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM firewall_rules WHERE server_id=$1`, s.ID); err != nil {
		return err
	}
	for i, r := range in.Rules {
		if _, err := tx.Exec(ctx, `INSERT INTO firewall_rules (server_id, pos, direction, action, proto, dport, addr, comment)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, s.ID, i, r.Direction, r.Action, r.Proto, r.Dport, r.Addr, r.Comment); err != nil {
			return err
		}
	}
	jobID, err := jobs.Enqueue(ctx, tx, s.ID, jobs.ActFirewall, jobs.Payload{})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	a.audit(ctx, c, "firewall_update", s.Hostname, map[string]any{"server_id": s.ID, "policy_in": in.PolicyIn, "rules": len(in.Rules)})
	return c.Status(202).JSON(fiber.Map{"job_id": jobID})
}

// ---------- Bekaplar ----------

type Backup struct {
	ID         int64      `json:"id"`
	ServerID   int64      `json:"server_id"`
	Hostname   string     `json:"hostname"`
	Note       string     `json:"note"`
	Status     string     `json:"status"`
	SizeBytes  int64      `json:"size_bytes"`
	Image      string     `json:"image"`
	Error      string     `json:"error"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

func (a *API) listBackups(c *fiber.Ctx) error {
	rows, err := a.DB.Query(c.Context(), `
		SELECT b.id, b.server_id, s.hostname, b.note, b.status, b.size_bytes, i.name || ' ' || i.version, b.error, b.created_at, b.finished_at
		FROM backups b
		JOIN servers s ON s.id=b.server_id
		JOIN images i ON i.id=b.image_id
		WHERE s.user_id=$1 AND s.deleted_at IS NULL AND b.status <> 'deleted'
		ORDER BY b.id DESC`, userOf(c).ID)
	if err != nil {
		return err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Backup, error) {
		var b Backup
		return b, r.Scan(&b.ID, &b.ServerID, &b.Hostname, &b.Note, &b.Status, &b.SizeBytes, &b.Image, &b.Error, &b.CreatedAt, &b.FinishedAt)
	})
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"limit": a.Cfg.BackupLimit, "backups": list})
}

func (a *API) createBackup(c *fiber.Ctx) error {
	s, err := a.ownServer(c)
	if err != nil {
		return err
	}
	if s.Status == "suspended" {
		return errSuspended
	}
	if s.Status != "running" && s.Status != "stopped" {
		return E(409, "bad_state", "Bekap diňe işleýän ýa-da durdurylan maşyndan alynýar")
	}
	var in struct {
		Note string `json:"note"`
	}
	if len(c.Body()) > 0 {
		if err := bind(c, &in); err != nil {
			return err
		}
	}
	note := cleanText(in.Note, 100)

	ctx := c.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE servers SET busy=true WHERE id=$1 AND NOT busy AND status IN ('running','stopped')`, s.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return E(409, "busy", "Maşynda başga amal ýerine ýetirilýär. Biraz garaşyň.")
	}
	// Maşyn "busy" bolany üçin şu barlag bilen goşmagyň arasynda başga bekap goşulyp bilmez
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM backups WHERE server_id=$1 AND status IN ('creating','ready','restoring','deleting')`, s.ID).Scan(&n); err != nil {
		return err
	}
	if n >= a.Cfg.BackupLimit {
		return E(409, "backup_limit", fmt.Sprintf("Bir maşyn üçin iň köp %d bekap saklanýar. Täze bekap üçin köne bekaby pozuň.", a.Cfg.BackupLimit))
	}
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO backups (server_id, image_id, note, storage)
		SELECT id, image_id, $2, $3 FROM servers WHERE id=$1 RETURNING id`, s.ID, note, a.Cfg.BackupStorage).Scan(&id); err != nil {
		return err
	}
	jobID, err := jobs.Enqueue(ctx, tx, s.ID, jobs.ActBackup, jobs.Payload{BackupID: id})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	a.audit(ctx, c, "backup_create", s.Hostname, map[string]any{"server_id": s.ID, "backup_id": id})
	return c.Status(202).JSON(fiber.Map{"id": id, "job_id": jobID})
}

// ownBackup ulanyja degişli bekabyň maşynyny we ýagdaýyny gaýtarýar.
func (a *API) ownBackup(c *fiber.Ctx) (id, serverID int64, status string, err error) {
	if id, err = idParam(c, "id"); err != nil {
		return
	}
	err = a.DB.QueryRow(c.Context(), `
		SELECT b.server_id, b.status FROM backups b JOIN servers s ON s.id=b.server_id
		WHERE b.id=$1 AND s.user_id=$2 AND s.deleted_at IS NULL AND b.status <> 'deleted'`, id, userOf(c).ID).Scan(&serverID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		err = E(404, "not_found", "Bekap tapylmady")
	}
	return
}

func (a *API) restoreBackup(c *fiber.Ctx) error {
	bid, sid, bstatus, err := a.ownBackup(c)
	if err != nil {
		return err
	}
	if bstatus != "ready" {
		return E(409, "backup_not_ready", "Bu bekap ulanmaga taýýar däl")
	}
	s, err := a.ownServerByID(c, sid)
	if err != nil {
		return err
	}
	if s.Status == "suspended" {
		return errSuspended
	}
	if s.Status != "running" && s.Status != "stopped" && s.Status != "error" {
		return E(409, "bad_state", "Bu ýagdaýda maşyny bekapdan dikeldip bolmaýar")
	}
	ctx := c.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE servers SET busy=true, status='restoring' WHERE id=$1 AND NOT busy AND status IN ('running','stopped','error')`, s.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return E(409, "busy", "Maşynda başga amal ýerine ýetirilýär. Biraz garaşyň.")
	}
	if tag, err = tx.Exec(ctx, `UPDATE backups SET status='restoring' WHERE id=$1 AND status='ready'`, bid); err != nil {
		return err
	} else if tag.RowsAffected() == 0 {
		return E(409, "backup_not_ready", "Bu bekap ulanmaga taýýar däl")
	}
	jobID, err := jobs.Enqueue(ctx, tx, s.ID, jobs.ActRestore, jobs.Payload{BackupID: bid})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	a.audit(ctx, c, "backup_restore", s.Hostname, map[string]any{"server_id": s.ID, "backup_id": bid})
	return c.Status(202).JSON(fiber.Map{"job_id": jobID})
}

func (a *API) deleteBackup(c *fiber.Ctx) error {
	bid, sid, bstatus, err := a.ownBackup(c)
	if err != nil {
		return err
	}
	if bstatus != "ready" && bstatus != "error" {
		return E(409, "busy", "Bu bekapda amal dowam edýär")
	}
	ctx := c.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE servers SET busy=true WHERE id=$1 AND NOT busy`, sid)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return E(409, "busy", "Maşynda başga amal ýerine ýetirilýär. Biraz garaşyň.")
	}
	if _, err := tx.Exec(ctx, `UPDATE backups SET status='deleting' WHERE id=$1`, bid); err != nil {
		return err
	}
	jobID, err := jobs.Enqueue(ctx, tx, sid, jobs.ActBackupDelete, jobs.Payload{BackupID: bid})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	a.audit(ctx, c, "backup_delete", "", map[string]any{"server_id": sid, "backup_id": bid})
	return c.Status(202).JSON(fiber.Map{"job_id": jobID})
}
