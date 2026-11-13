package httpapi

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"github.com/Esca6585dev/os-astu-tm/api/internal/billing"
	"github.com/Esca6585dev/os-astu-tm/api/internal/security"
)

type Plan struct {
	ID            int64  `json:"id"`
	Code          string `json:"code"`
	VCPU          int    `json:"vcpu"`
	RAMMB         int    `json:"ram_mb"`
	DiskGB        int    `json:"disk_gb"`
	IPCount       int    `json:"ip_count"`
	BandwidthMbit int    `json:"bandwidth_mbit"`
	PriceMonthly  int64  `json:"price_monthly"`
	PriceHourly   int64  `json:"price_hourly"` // tegeleklenen, diňe görkezmek üçin
	IsPopular     bool   `json:"is_popular"`
	IsActive      bool   `json:"is_active"`
	Sort          int    `json:"sort"`
}

const planCols = `id, code, vcpu, ram_mb, disk_gb, ip_count, bandwidth_mbit, price_monthly, is_popular, is_active, sort`

func scanPlans(rows pgx.Rows) ([]Plan, error) {
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Plan, error) {
		var p Plan
		err := r.Scan(&p.ID, &p.Code, &p.VCPU, &p.RAMMB, &p.DiskGB, &p.IPCount, &p.BandwidthMbit, &p.PriceMonthly, &p.IsPopular, &p.IsActive, &p.Sort)
		p.PriceHourly = (p.PriceMonthly*2 + billing.HoursPerMonth) / (2 * billing.HoursPerMonth)
		return p, err
	})
}

func (a *API) listPlans(c *fiber.Ctx) error {
	rows, err := a.DB.Query(c.Context(), `SELECT `+planCols+` FROM plans WHERE is_active ORDER BY sort, price_monthly`)
	if err != nil {
		return err
	}
	ps, err := scanPlans(rows)
	if err != nil {
		return err
	}
	return c.JSON(ps)
}

type Image struct {
	ID           int64  `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	TemplateVMID int    `json:"proxmox_template_vmid,omitempty"`
	IsActive     bool   `json:"is_active"`
	Sort         int    `json:"sort"`
}

func scanImages(rows pgx.Rows, withTpl bool) ([]Image, error) {
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Image, error) {
		var i Image
		err := r.Scan(&i.ID, &i.Slug, &i.Name, &i.Version, &i.TemplateVMID, &i.IsActive, &i.Sort)
		if !withTpl {
			i.TemplateVMID = 0
		}
		return i, err
	})
}

func (a *API) listImages(c *fiber.Ctx) error {
	rows, err := a.DB.Query(c.Context(), `SELECT id, slug, name, version, proxmox_template_vmid, is_active, sort FROM images WHERE is_active ORDER BY sort, id`)
	if err != nil {
		return err
	}
	is, err := scanImages(rows, false)
	if err != nil {
		return err
	}
	return c.JSON(is)
}

// ---------- SSH açarlar ----------

type SSHKey struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
	UsedBy      []string  `json:"used_by"`
}

func (a *API) listKeys(c *fiber.Ctx) error {
	rows, err := a.DB.Query(c.Context(), `
		SELECT k.id, k.name, k.key_type, k.fingerprint, k.created_at,
		       COALESCE(array_agg(s.hostname ORDER BY s.hostname) FILTER (WHERE s.id IS NOT NULL), '{}')
		FROM ssh_keys k
		LEFT JOIN server_ssh_keys sk ON sk.ssh_key_id=k.id
		LEFT JOIN servers s ON s.id=sk.server_id AND s.deleted_at IS NULL
		WHERE k.user_id=$1 GROUP BY k.id ORDER BY k.created_at DESC`, userOf(c).ID)
	if err != nil {
		return err
	}
	ks, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (SSHKey, error) {
		var k SSHKey
		return k, r.Scan(&k.ID, &k.Name, &k.Type, &k.Fingerprint, &k.CreatedAt, &k.UsedBy)
	})
	if err != nil {
		return err
	}
	return c.JSON(ks)
}

func (a *API) addKey(c *fiber.Ctx) error {
	var in struct {
		Name      string `json:"name"`
		PublicKey string `json:"public_key"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	in.Name = trim(in.Name)
	if in.Name == "" || len(in.Name) > 60 {
		return E(422, "invalid_name", "Açaryň adyny ýazyň (60 simwoldan az)")
	}
	k, err := security.ParseSSHKey(in.PublicKey)
	if err != nil {
		return E(422, "invalid_key", err.Error())
	}
	var id int64
	err = a.DB.QueryRow(c.Context(), `INSERT INTO ssh_keys (user_id, name, public_key, key_type, fingerprint) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id, fingerprint) DO NOTHING RETURNING id`, userOf(c).ID, in.Name, k.Normalized, k.Type, k.Fingerprint).Scan(&id)
	if err == pgx.ErrNoRows {
		return E(409, "duplicate_key", "Bu açar eýýäm goşulan")
	}
	if err != nil {
		return err
	}
	return c.Status(201).JSON(fiber.Map{"id": id, "fingerprint": k.Fingerprint})
}

func (a *API) deleteKey(c *fiber.Ctx) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	tag, err := a.DB.Exec(c.Context(), `DELETE FROM ssh_keys WHERE id=$1 AND user_id=$2`, id, userOf(c).ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return E(404, "not_found", "Tapylmady")
	}
	return c.JSON(fiber.Map{"ok": true})
}

// ---------- Balans ----------

func (a *API) getBalance(c *fiber.Ctx) error {
	u := userOf(c)
	var monthly, used, servers int64
	monthStart := time.Now().In(time.Local)
	monthStart = time.Date(monthStart.Year(), monthStart.Month(), 1, 0, 0, 0, 0, time.Local)
	if err := a.DB.QueryRow(c.Context(), `
		SELECT COALESCE(sum(p.price_monthly),0), count(*)
		FROM servers s JOIN plans p ON p.id=s.plan_id
		WHERE s.user_id=$1 AND s.status IN ('provisioning','running','stopped','reinstalling','restoring')`, u.ID).Scan(&monthly, &servers); err != nil {
		return err
	}
	if err := a.DB.QueryRow(c.Context(), `
		SELECT COALESCE(sum(-amount),0) FROM transactions WHERE user_id=$1 AND type='charge' AND created_at >= $2`, u.ID, monthStart).Scan(&used); err != nil {
		return err
	}
	hourly := float64(monthly) / billing.HoursPerMonth
	var days any = nil
	if hourly > 0 && u.Balance > 0 {
		days = int64(float64(u.Balance) / (hourly * 24))
	}
	return c.JSON(fiber.Map{
		"balance":          u.Balance,
		"hourly":           int64(hourly + 0.5),
		"monthly_cap":      monthly,
		"month_used":       used,
		"days_left":        days,
		"billable_servers": servers,
		"account_no":       u.Account,
		"payment_info":     a.Cfg.PaymentInfo,
	})
}

type Tx struct {
	ID           int64     `json:"id"`
	Type         string    `json:"type"`
	Amount       int64     `json:"amount"`
	BalanceAfter int64     `json:"balance_after"`
	Note         string    `json:"note"`
	ServerID     *int64    `json:"server_id"`
	CreatedAt    time.Time `json:"created_at"`
	UserEmail    string    `json:"user_email,omitempty"`
}

func (a *API) queryTx(c *fiber.Ctx, userID int64) error {
	limit, offset := page(c)
	typ := c.Query("type")
	rows, err := a.DB.Query(c.Context(), `
		SELECT t.id, t.type, t.amount, t.balance_after, t.note, t.server_id, t.created_at, u.email
		FROM transactions t JOIN users u ON u.id=t.user_id
		WHERE ($1::bigint = 0 OR t.user_id=$1) AND ($2 = '' OR t.type=$2)
		ORDER BY t.created_at DESC, t.id DESC LIMIT $3 OFFSET $4`, userID, typ, limit, offset)
	if err != nil {
		return err
	}
	txs, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Tx, error) {
		var t Tx
		return t, r.Scan(&t.ID, &t.Type, &t.Amount, &t.BalanceAfter, &t.Note, &t.ServerID, &t.CreatedAt, &t.UserEmail)
	})
	if err != nil {
		return err
	}
	if userID != 0 {
		for i := range txs {
			txs[i].UserEmail = ""
		}
	}
	return c.JSON(txs)
}

func (a *API) listTransactions(c *fiber.Ctx) error { return a.queryTx(c, userOf(c).ID) }
