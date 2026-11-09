package httpapi

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"github.com/Esca6585dev/os-astu-tm/api/internal/billing"
	"github.com/Esca6585dev/os-astu-tm/api/internal/hv"
	"github.com/Esca6585dev/os-astu-tm/api/internal/jobs"
	"github.com/Esca6585dev/os-astu-tm/api/internal/security"
)

var hostnameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

type Server struct {
	ID           int64      `json:"id"`
	Hostname     string     `json:"hostname"`
	Status       string     `json:"status"`
	Busy         bool       `json:"busy"`
	IP           *string    `json:"ip"`
	Gateway      *string    `json:"gateway"`
	Plan         Plan       `json:"plan"`
	Image        Image      `json:"image"`
	Node         string     `json:"node"`
	VMID         *int       `json:"vmid"`
	CreatedAt    time.Time  `json:"created_at"`
	SuspendedAt  *time.Time `json:"suspended_at"`
	MonthUsed    int64      `json:"month_used"`
	UserEmail    string     `json:"user_email,omitempty"`
	Usage        *hv.Usage  `json:"usage,omitempty"`
	LastJobError *string    `json:"last_job_error"`
}

const serverSelect = `
	SELECT s.id, s.hostname, s.status, s.busy, a.address, ip.gateway,
	       p.id, p.code, p.vcpu, p.ram_mb, p.disk_gb, p.ip_count, p.bandwidth_mbit, p.price_monthly, p.is_popular, p.is_active, p.sort,
	       i.id, i.slug, i.name, i.version,
	       n.name, s.proxmox_vmid, s.created_at, s.suspended_at,
	       COALESCE((SELECT sum(-t.amount) FROM transactions t WHERE t.server_id=s.id AND t.type='charge' AND t.created_at >= date_trunc('month', now())),0),
	       u.email,
	       (SELECT NULLIF(j.error,'') FROM jobs j WHERE j.server_id=s.id ORDER BY j.id DESC LIMIT 1)
	FROM servers s
	JOIN users u ON u.id=s.user_id
	JOIN plans p ON p.id=s.plan_id
	JOIN images i ON i.id=s.image_id
	JOIN nodes n ON n.id=s.node_id
	LEFT JOIN ip_addresses a ON a.server_id=s.id
	LEFT JOIN ip_pools ip ON ip.id=a.pool_id`

func scanServer(r pgx.Row) (Server, error) {
	var s Server
	p := &s.Plan
	err := r.Scan(&s.ID, &s.Hostname, &s.Status, &s.Busy, &s.IP, &s.Gateway,
		&p.ID, &p.Code, &p.VCPU, &p.RAMMB, &p.DiskGB, &p.IPCount, &p.BandwidthMbit, &p.PriceMonthly, &p.IsPopular, &p.IsActive, &p.Sort,
		&s.Image.ID, &s.Image.Slug, &s.Image.Name, &s.Image.Version,
		&s.Node, &s.VMID, &s.CreatedAt, &s.SuspendedAt, &s.MonthUsed, &s.UserEmail, &s.LastJobError)
	p.PriceHourly = (p.PriceMonthly*2 + billing.HoursPerMonth) / (2 * billing.HoursPerMonth)
	s.Image.IsActive = true
	return s, err
}

func (a *API) listServers(c *fiber.Ctx) error {
	rows, err := a.DB.Query(c.Context(), serverSelect+` WHERE s.user_id=$1 AND s.deleted_at IS NULL ORDER BY s.created_at DESC`, userOf(c).ID)
	if err != nil {
		return err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Server, error) { return scanServer(r) })
	if err != nil {
		return err
	}
	for i := range list {
		list[i].UserEmail = ""
	}
	return c.JSON(list)
}

func (a *API) ownServer(c *fiber.Ctx) (Server, error) {
	id, err := idParam(c, "id")
	if err != nil {
		return Server{}, err
	}
	s, err := scanServer(a.DB.QueryRow(c.Context(), serverSelect+` WHERE s.id=$1 AND s.user_id=$2 AND s.status <> 'deleted'`, id, userOf(c).ID))
	if errors.Is(err, pgx.ErrNoRows) {
		return s, E(404, "not_found", "Maşyn tapylmady")
	}
	s.UserEmail = ""
	return s, err
}

func (a *API) getServer(c *fiber.Ctx) error {
	s, err := a.ownServer(c)
	if err != nil {
		return err
	}
	if s.VMID != nil && (s.Status == "running" || s.Status == "stopped") {
		ctx, cancel := context.WithTimeout(c.Context(), 4*time.Second)
		defer cancel()
		if u, err := a.HV.Usage(ctx, s.Node, *s.VMID); err == nil {
			s.Usage = &u
		}
	}
	return c.JSON(s)
}

func (a *API) createServer(c *fiber.Ctx) error {
	u := userOf(c)
	if u.Status != "active" {
		return E(403, "inactive", "Maşyn döretmek üçin emailiňizi tassyklaň")
	}
	var in struct {
		PlanID    int64   `json:"plan_id"`
		ImageID   int64   `json:"image_id"`
		Hostname  string  `json:"hostname"`
		SSHKeyIDs []int64 `json:"ssh_key_ids"`
		Password  string  `json:"password"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	in.Hostname = trim(in.Hostname)
	if !hostnameRe.MatchString(in.Hostname) {
		return E(422, "invalid_hostname", "Hostname diňe kiçi latyn harplaryndan, sanlardan we “-” belgisinden durmaly")
	}
	if in.Password != "" && (len(in.Password) < 12 || len(in.Password) > 72) {
		return E(422, "weak_password", "Root parol 12–72 simwol bolmaly")
	}
	generated := ""
	if len(in.SSHKeyIDs) == 0 && in.Password == "" {
		generated = security.RandomPassword(16)
		in.Password = generated
	}

	ctx := c.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var vcpu, ram, disk int
	var price int64
	err = tx.QueryRow(ctx, `SELECT vcpu, ram_mb, disk_gb, price_monthly FROM plans WHERE id=$1 AND is_active`, in.PlanID).Scan(&vcpu, &ram, &disk, &price)
	if errors.Is(err, pgx.ErrNoRows) {
		return E(422, "invalid_plan", "Konfigurasiýa tapylmady")
	} else if err != nil {
		return err
	}
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM images WHERE id=$1 AND is_active)`, in.ImageID).Scan(&ok); err != nil {
		return err
	} else if !ok {
		return E(422, "invalid_image", "Obraz tapylmady")
	}

	// Balansy gulplap barlamak (iki parallel sorag bir balansy iki gezek ulanmasyn)
	var balance int64
	if err := tx.QueryRow(ctx, `SELECT balance FROM users WHERE id=$1 FOR UPDATE`, u.ID).Scan(&balance); err != nil {
		return err
	}
	var committed int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(p.price_monthly),0) FROM servers s JOIN plans p ON p.id=s.plan_id
		WHERE s.user_id=$1 AND s.status IN ('provisioning','running','stopped','reinstalling')`, u.ID).Scan(&committed); err != nil {
		return err
	}
	need := (committed + price) * a.Cfg.PVEMinCreditHrs / billing.HoursPerMonth
	if balance < need {
		return E(402, "insufficient_balance", "Balans ýeterlik däl: ähli maşynlaryňyz üçin azyndan "+
			billing.FormatTMT(need)+" TMT (24 sagatlyk tutum) gerek")
	}

	// SSH açarlar ulanyja degişlimi
	if len(in.SSHKeyIDs) > 0 {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM ssh_keys WHERE user_id=$1 AND id = ANY($2)`, u.ID, in.SSHKeyIDs).Scan(&n); err != nil {
			return err
		}
		if n != len(in.SSHKeyIDs) {
			return E(422, "invalid_keys", "SSH açar tapylmady")
		}
	}

	// Ýeterlik kuwwaty we boş IP-si bolan node
	var nodeID int64
	err = tx.QueryRow(ctx, `
		SELECT n.id FROM nodes n
		LEFT JOIN LATERAL (
			SELECT COALESCE(sum(p.vcpu),0) cpu, COALESCE(sum(p.ram_mb),0) ram, COALESCE(sum(p.disk_gb),0) disk
			FROM servers s JOIN plans p ON p.id=s.plan_id
			WHERE s.node_id=n.id AND s.status <> 'deleted') used ON true
		WHERE n.is_active
		  AND used.cpu + $1 <= n.max_vcpu AND used.ram + $2 <= n.max_ram_mb AND used.disk + $3 <= n.max_disk_gb
		  AND EXISTS (SELECT 1 FROM ip_addresses a JOIN ip_pools ip ON ip.id=a.pool_id
		              WHERE ip.node_id=n.id AND a.server_id IS NULL AND NOT a.reserved)
		ORDER BY (n.max_ram_mb - used.ram) DESC
		LIMIT 1 FOR UPDATE OF n`, vcpu, ram, disk).Scan(&nodeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return E(503, "no_capacity", "Häzirki wagtda boş kuwwat ýa-da IP salgy ýok. Administrator bilen habarlaşyň.")
	} else if err != nil {
		return err
	}

	var sid int64
	if err := tx.QueryRow(ctx, `INSERT INTO servers (user_id, plan_id, image_id, node_id, hostname, busy)
		VALUES ($1,$2,$3,$4,$5,true) RETURNING id`, u.ID, in.PlanID, in.ImageID, nodeID, in.Hostname).Scan(&sid); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE ip_addresses SET server_id=$1 WHERE id = (
		SELECT a.id FROM ip_addresses a JOIN ip_pools ip ON ip.id=a.pool_id
		WHERE ip.node_id=$2 AND a.server_id IS NULL AND NOT a.reserved
		ORDER BY a.id LIMIT 1 FOR UPDATE SKIP LOCKED)`, sid, nodeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return E(503, "no_ip", "Boş IP salgy ýok")
	}
	for _, k := range in.SSHKeyIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO server_ssh_keys (server_id, ssh_key_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, sid, k); err != nil {
			return err
		}
	}
	jobID, err := jobs.Enqueue(ctx, tx, sid, jobs.ActCreate, jobs.Payload{Password: in.Password})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	a.audit(ctx, c, "server_create", in.Hostname, map[string]any{"server_id": sid, "plan_id": in.PlanID})

	resp := fiber.Map{"id": sid, "job_id": jobID}
	if generated != "" {
		resp["root_password"] = generated
	}
	return c.Status(202).JSON(resp)
}

// lockForAction maşyny "busy" edýär we job goşýar.
func (a *API) lockForAction(ctx context.Context, sid int64, action string, p jobs.Payload, extraSQL string) (int64, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE servers SET busy=true`+extraSQL+` WHERE id=$1 AND NOT busy`, sid)
	if err != nil {
		return 0, err
	}
	if tag.RowsAffected() == 0 {
		return 0, E(409, "busy", "Maşynda başga amal ýerine ýetirilýär. Biraz garaşyň.")
	}
	id, err := jobs.Enqueue(ctx, tx, sid, action, p)
	if err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

func (a *API) serverAction(c *fiber.Ctx) error {
	s, err := a.ownServer(c)
	if err != nil {
		return err
	}
	if s.Status == "suspended" {
		return E(402, "suspended", "Maşyn balans gutaranlygy sebäpli saklanýar. Balansy dolduryň.")
	}
	if s.Busy {
		return E(409, "busy", "Maşynda başga amal ýerine ýetirilýär. Biraz garaşyň.")
	}
	ctx := c.Context()
	resp := fiber.Map{}
	var jobID int64
	switch c.Params("action") {
	case "start":
		if s.Status != "stopped" {
			return E(409, "bad_state", "Maşyn durdurylan däl")
		}
		jobID, err = a.lockForAction(ctx, s.ID, jobs.ActStart, jobs.Payload{}, "")
	case "stop":
		if s.Status != "running" {
			return E(409, "bad_state", "Maşyn işlemeýär")
		}
		jobID, err = a.lockForAction(ctx, s.ID, jobs.ActStop, jobs.Payload{}, "")
	case "reboot":
		if s.Status != "running" {
			return E(409, "bad_state", "Maşyn işlemeýär")
		}
		jobID, err = a.lockForAction(ctx, s.ID, jobs.ActReboot, jobs.Payload{}, "")
	case "reset-password":
		if s.Status != "running" {
			return E(409, "bad_state", "Paroly täzelemek üçin maşyn işläp durmaly")
		}
		pw := security.RandomPassword(16)
		jobID, err = a.lockForAction(ctx, s.ID, jobs.ActResetPassword, jobs.Payload{Password: pw}, "")
		resp["root_password"] = pw
	case "reinstall":
		if s.Status != "running" && s.Status != "stopped" && s.Status != "error" {
			return E(409, "bad_state", "Bu ýagdaýda OS-i täzeden gurup bolmaýar")
		}
		var in struct {
			ImageID  int64  `json:"image_id"`
			Password string `json:"password"`
		}
		if err := bind(c, &in); err != nil {
			return err
		}
		var ok bool
		if err := a.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM images WHERE id=$1 AND is_active)`, in.ImageID).Scan(&ok); err != nil || !ok {
			return E(422, "invalid_image", "Obraz tapylmady")
		}
		var keys int
		_ = a.DB.QueryRow(ctx, `SELECT count(*) FROM server_ssh_keys WHERE server_id=$1`, s.ID).Scan(&keys)
		if in.Password == "" && keys == 0 {
			in.Password = security.RandomPassword(16)
			resp["root_password"] = in.Password
		}
		jobID, err = a.lockForAction(ctx, s.ID, jobs.ActReinstall, jobs.Payload{ImageID: in.ImageID, Password: in.Password}, ", status='reinstalling'")
	default:
		return E(404, "not_found", "Näbelli amal")
	}
	if err != nil {
		return err
	}
	a.audit(ctx, c, "server_"+c.Params("action"), s.Hostname, map[string]any{"server_id": s.ID})
	resp["job_id"] = jobID
	return c.Status(202).JSON(resp)
}

func (a *API) deleteServer(c *fiber.Ctx) error {
	s, err := a.ownServer(c)
	if err != nil {
		return err
	}
	if s.Status == "deleting" {
		return c.Status(202).JSON(fiber.Map{"ok": true})
	}
	jobID, err := a.lockForAction(c.Context(), s.ID, jobs.ActDelete, jobs.Payload{}, ", status='deleting', deleted_at=now()")
	if err != nil {
		return err
	}
	a.audit(c.Context(), c, "server_delete", s.Hostname, map[string]any{"server_id": s.ID})
	return c.Status(202).JSON(fiber.Map{"job_id": jobID})
}

func (a *API) serverJobs(c *fiber.Ctx) error {
	s, err := a.ownServer(c)
	if err != nil {
		return err
	}
	rows, err := a.DB.Query(c.Context(), `SELECT id, action, status, error, created_at, finished_at FROM jobs WHERE server_id=$1 ORDER BY id DESC LIMIT 10`, s.ID)
	if err != nil {
		return err
	}
	type J struct {
		ID         int64      `json:"id"`
		Action     string     `json:"action"`
		Status     string     `json:"status"`
		Error      string     `json:"error"`
		CreatedAt  time.Time  `json:"created_at"`
		FinishedAt *time.Time `json:"finished_at"`
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (J, error) {
		var j J
		err := r.Scan(&j.ID, &j.Action, &j.Status, &j.Error, &j.CreatedAt, &j.FinishedAt)
		j.Error = jobs.ShortError(j.Error)
		return j, err
	})
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"busy": s.Busy, "status": s.Status, "jobs": list})
}
