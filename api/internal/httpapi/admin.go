package httpapi

import (
	"errors"
	"math"
	"net/netip"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (a *API) adminStats(c *fiber.Ctx) error {
	ctx := c.Context()
	out := fiber.Map{}
	var users, running, servers, freeIPs, totalIPs int64
	if err := a.DB.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM users),
		(SELECT count(*) FROM servers WHERE status='running'),
		(SELECT count(*) FROM servers WHERE deleted_at IS NULL),
		(SELECT count(*) FROM ip_addresses WHERE server_id IS NULL AND NOT reserved),
		(SELECT count(*) FROM ip_addresses)`).Scan(&users, &running, &servers, &freeIPs, &totalIPs); err != nil {
		return err
	}
	out["users"], out["running_servers"], out["servers"], out["free_ips"], out["total_ips"] = users, running, servers, freeIPs, totalIPs

	rows, err := a.DB.Query(ctx, `
		SELECT n.id, n.name, n.max_vcpu, n.max_ram_mb, n.max_disk_gb,
		       COALESCE(sum(p.vcpu),0), COALESCE(sum(p.ram_mb),0), COALESCE(sum(p.disk_gb),0)
		FROM nodes n LEFT JOIN servers s ON s.node_id=n.id AND s.status <> 'deleted'
		LEFT JOIN plans p ON p.id=s.plan_id
		GROUP BY n.id ORDER BY n.name`)
	if err != nil {
		return err
	}
	type NodeUse struct {
		ID                          int64  `json:"id"`
		Name                        string `json:"name"`
		MaxVCPU, MaxRAM, MaxDisk    int64
		UsedVCPU, UsedRAM, UsedDisk int64
	}
	nodes, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (map[string]any, error) {
		var n NodeUse
		err := r.Scan(&n.ID, &n.Name, &n.MaxVCPU, &n.MaxRAM, &n.MaxDisk, &n.UsedVCPU, &n.UsedRAM, &n.UsedDisk)
		return map[string]any{"id": n.ID, "name": n.Name,
			"vcpu": [2]int64{n.UsedVCPU, n.MaxVCPU}, "ram_mb": [2]int64{n.UsedRAM, n.MaxRAM}, "disk_gb": [2]int64{n.UsedDisk, n.MaxDisk}}, err
	})
	if err != nil {
		return err
	}
	out["nodes"] = nodes
	return c.JSON(out)
}

func (a *API) adminUsers(c *fiber.Ctx) error {
	limit, offset := page(c)
	q := "%" + strings.ToLower(trim(c.Query("q"))) + "%"
	rows, err := a.DB.Query(c.Context(), `
		SELECT u.id, u.email, u.full_name, u.phone, u.role, u.status, u.balance, u.email_verified_at IS NOT NULL,
		       (SELECT count(*) FROM servers s WHERE s.user_id=u.id AND s.deleted_at IS NULL), u.created_at
		FROM users u
		WHERE lower(u.email::text) LIKE $1 OR lower(u.full_name) LIKE $1 OR ('u-' || (10000+u.id)::text) LIKE $1
		ORDER BY u.id DESC LIMIT $2 OFFSET $3`, q, limit, offset)
	if err != nil {
		return err
	}
	type AU struct {
		User
		Servers   int64     `json:"servers"`
		CreatedAt time.Time `json:"created_at"`
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (AU, error) {
		var u AU
		err := r.Scan(&u.ID, &u.Email, &u.FullName, &u.Phone, &u.Role, &u.Status, &u.Balance, &u.Verified, &u.Servers, &u.CreatedAt)
		u.Account = accountNo(u.ID)
		return u, err
	})
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) adminPatchUser(c *fiber.Ctx) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Status *string `json:"status"`
		Role   *string `json:"role"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	if in.Status != nil && *in.Status != "active" && *in.Status != "blocked" && *in.Status != "pending" {
		return E(422, "invalid_status", "Nädogry ýagdaý")
	}
	if in.Role != nil && *in.Role != "user" && *in.Role != "admin" {
		return E(422, "invalid_role", "Nädogry rol")
	}
	if id == userOf(c).ID && ((in.Role != nil && *in.Role != "admin") || (in.Status != nil && *in.Status != "active")) {
		return E(422, "self", "Öz hasabyňyzy bloklap ýa-da admin hukugyny aýryp bilmeýärsiňiz")
	}
	tag, err := a.DB.Exec(c.Context(), `UPDATE users SET status=COALESCE($2,status), role=COALESCE($3,role) WHERE id=$1`, id, in.Status, in.Role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return E(404, "not_found", "Ulanyjy tapylmady")
	}
	if in.Status != nil && *in.Status == "blocked" {
		_, _ = a.DB.Exec(c.Context(), `DELETE FROM sessions WHERE user_id=$1`, id)
	}
	a.audit(c.Context(), c, "user_update", accountNo(id), map[string]any{"status": in.Status, "role": in.Role})
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) adminTopup(c *fiber.Ctx) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Amount float64 `json:"amount"` // TMT
		Note   string  `json:"note"`
		Type   string  `json:"type"` // topup (default) | adjustment | refund
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	tenge := int64(math.Round(in.Amount * 100))
	if in.Type == "" {
		in.Type = "topup"
	}
	if in.Type != "topup" && in.Type != "adjustment" && in.Type != "refund" {
		return E(422, "invalid_type", "Nädogry görnüş")
	}
	if tenge == 0 || math.Abs(in.Amount) > 1_000_000 || (in.Type != "adjustment" && tenge < 0) {
		return E(422, "invalid_amount", "Dogry möçber giriziň")
	}
	in.Note = truncate(trim(in.Note), 200)
	if in.Note == "" {
		return E(422, "note_required", "Bellik (çek ýa-da töleg belgisi) hökmany")
	}
	ctx := c.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var bal int64
	err = tx.QueryRow(ctx, `UPDATE users SET balance=balance+$2, low_balance_notified_at=NULL WHERE id=$1 RETURNING balance`, id, tenge).Scan(&bal)
	if errors.Is(err, pgx.ErrNoRows) {
		return E(404, "not_found", "Ulanyjy tapylmady")
	} else if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO transactions (user_id, type, amount, balance_after, admin_id, note) VALUES ($1,$2,$3,$4,$5,$6)`,
		id, in.Type, tenge, bal, userOf(c).ID, in.Note); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	a.audit(ctx, c, "balance_"+in.Type, accountNo(id), map[string]any{"amount": tenge, "note": in.Note})
	return c.JSON(fiber.Map{"balance": bal})
}

func (a *API) adminServers(c *fiber.Ctx) error {
	limit, offset := page(c)
	q := "%" + strings.ToLower(trim(c.Query("q"))) + "%"
	rows, err := a.DB.Query(c.Context(), serverSelect+`
		WHERE s.status <> 'deleted' AND (lower(s.hostname) LIKE $1 OR lower(u.email::text) LIKE $1 OR COALESCE(a.address,'') LIKE $1)
		ORDER BY s.created_at DESC LIMIT $2 OFFSET $3`, q, limit, offset)
	if err != nil {
		return err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Server, error) { return scanServer(r) })
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) adminTransactions(c *fiber.Ctx) error {
	return a.queryTx(c, int64(c.QueryInt("user_id", 0)))
}

func (a *API) adminAudit(c *fiber.Ctx) error {
	limit, offset := page(c)
	rows, err := a.DB.Query(c.Context(), `
		SELECT l.id, COALESCE(u.email::text,''), l.action, l.target, l.meta, l.ip, l.created_at
		FROM audit_log l LEFT JOIN users u ON u.id=l.actor_id ORDER BY l.id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return err
	}
	type L struct {
		ID        int64          `json:"id"`
		Actor     string         `json:"actor"`
		Action    string         `json:"action"`
		Target    string         `json:"target"`
		Meta      map[string]any `json:"meta"`
		IP        string         `json:"ip"`
		CreatedAt time.Time      `json:"created_at"`
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (L, error) {
		var l L
		return l, r.Scan(&l.ID, &l.Actor, &l.Action, &l.Target, &l.Meta, &l.IP, &l.CreatedAt)
	})
	if err != nil {
		return err
	}
	return c.JSON(list)
}

// ---------- Tarifler, obrazlar, node-lar ----------

func (a *API) adminPlans(c *fiber.Ctx) error {
	rows, err := a.DB.Query(c.Context(), `SELECT `+planCols+` FROM plans ORDER BY sort, id`)
	if err != nil {
		return err
	}
	ps, err := scanPlans(rows)
	if err != nil {
		return err
	}
	return c.JSON(ps)
}

func (a *API) adminSavePlan(c *fiber.Ctx) error {
	var p Plan
	if err := bind(c, &p); err != nil {
		return err
	}
	if trim(p.Code) == "" || p.VCPU < 1 || p.RAMMB < 256 || p.DiskGB < 5 || p.BandwidthMbit < 1 || p.PriceMonthly < 0 {
		return E(422, "invalid_plan", "Konfigurasiýa maglumatlary nädogry (baha teňňede)")
	}
	if p.IPCount < 1 {
		p.IPCount = 1
	}
	var err error
	if c.Params("id") == "" {
		err = a.DB.QueryRow(c.Context(), `INSERT INTO plans (code, vcpu, ram_mb, disk_gb, ip_count, bandwidth_mbit, price_monthly, is_popular, is_active, sort)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, p.Code, p.VCPU, p.RAMMB, p.DiskGB, p.IPCount, p.BandwidthMbit, p.PriceMonthly, p.IsPopular, p.IsActive, p.Sort).Scan(&p.ID)
	} else {
		if p.ID, err = idParam(c, "id"); err != nil {
			return err
		}
		_, err = a.DB.Exec(c.Context(), `UPDATE plans SET code=$2, vcpu=$3, ram_mb=$4, disk_gb=$5, ip_count=$6, bandwidth_mbit=$7, price_monthly=$8, is_popular=$9, is_active=$10, sort=$11 WHERE id=$1`,
			p.ID, p.Code, p.VCPU, p.RAMMB, p.DiskGB, p.IPCount, p.BandwidthMbit, p.PriceMonthly, p.IsPopular, p.IsActive, p.Sort)
	}
	if err != nil {
		return err
	}
	a.audit(c.Context(), c, "plan_save", p.Code, map[string]any{"price_monthly": p.PriceMonthly})
	return c.JSON(p)
}

func (a *API) adminImages(c *fiber.Ctx) error {
	rows, err := a.DB.Query(c.Context(), `SELECT id, slug, name, version, proxmox_template_vmid, is_active, sort FROM images ORDER BY sort, id`)
	if err != nil {
		return err
	}
	is, err := scanImages(rows, true)
	if err != nil {
		return err
	}
	return c.JSON(is)
}

func (a *API) adminSaveImage(c *fiber.Ctx) error {
	var i Image
	if err := bind(c, &i); err != nil {
		return err
	}
	if trim(i.Slug) == "" || trim(i.Name) == "" || i.TemplateVMID < 100 {
		return E(422, "invalid_image", "slug, name we proxmox_template_vmid (≥100) gerek")
	}
	var err error
	if c.Params("id") == "" {
		err = a.DB.QueryRow(c.Context(), `INSERT INTO images (slug, name, version, proxmox_template_vmid, is_active, sort) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
			i.Slug, i.Name, i.Version, i.TemplateVMID, i.IsActive, i.Sort).Scan(&i.ID)
	} else {
		if i.ID, err = idParam(c, "id"); err != nil {
			return err
		}
		_, err = a.DB.Exec(c.Context(), `UPDATE images SET slug=$2, name=$3, version=$4, proxmox_template_vmid=$5, is_active=$6, sort=$7 WHERE id=$1`,
			i.ID, i.Slug, i.Name, i.Version, i.TemplateVMID, i.IsActive, i.Sort)
	}
	if err != nil {
		return err
	}
	a.audit(c.Context(), c, "image_save", i.Slug, nil)
	return c.JSON(i)
}

type Node struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Storage   string `json:"storage"`
	Bridge    string `json:"bridge"`
	MaxVCPU   int    `json:"max_vcpu"`
	MaxRAMMB  int    `json:"max_ram_mb"`
	MaxDiskGB int    `json:"max_disk_gb"`
	IsActive  bool   `json:"is_active"`
}

func (a *API) adminNodes(c *fiber.Ctx) error {
	rows, err := a.DB.Query(c.Context(), `SELECT id, name, storage, bridge, max_vcpu, max_ram_mb, max_disk_gb, is_active FROM nodes ORDER BY name`)
	if err != nil {
		return err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Node, error) {
		var n Node
		return n, r.Scan(&n.ID, &n.Name, &n.Storage, &n.Bridge, &n.MaxVCPU, &n.MaxRAMMB, &n.MaxDiskGB, &n.IsActive)
	})
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) adminSaveNode(c *fiber.Ctx) error {
	var n Node
	if err := bind(c, &n); err != nil {
		return err
	}
	if trim(n.Name) == "" || n.MaxVCPU < 1 || n.MaxRAMMB < 1024 || n.MaxDiskGB < 10 {
		return E(422, "invalid_node", "Node maglumatlary nädogry")
	}
	if n.Storage == "" {
		n.Storage = "local-lvm"
	}
	if n.Bridge == "" {
		n.Bridge = "vmbr0"
	}
	var err error
	if c.Params("id") == "" {
		err = a.DB.QueryRow(c.Context(), `INSERT INTO nodes (name, storage, bridge, max_vcpu, max_ram_mb, max_disk_gb, is_active) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
			n.Name, n.Storage, n.Bridge, n.MaxVCPU, n.MaxRAMMB, n.MaxDiskGB, n.IsActive).Scan(&n.ID)
	} else {
		if n.ID, err = idParam(c, "id"); err != nil {
			return err
		}
		_, err = a.DB.Exec(c.Context(), `UPDATE nodes SET name=$2, storage=$3, bridge=$4, max_vcpu=$5, max_ram_mb=$6, max_disk_gb=$7, is_active=$8 WHERE id=$1`,
			n.ID, n.Name, n.Storage, n.Bridge, n.MaxVCPU, n.MaxRAMMB, n.MaxDiskGB, n.IsActive)
	}
	if err != nil {
		return err
	}
	a.audit(c.Context(), c, "node_save", n.Name, nil)
	return c.JSON(n)
}

func (a *API) adminPools(c *fiber.Ctx) error {
	rows, err := a.DB.Query(c.Context(), `
		SELECT p.id, n.name, p.cidr, p.gateway, p.dns,
		       count(a.id), count(a.id) FILTER (WHERE a.server_id IS NULL AND NOT a.reserved)
		FROM ip_pools p JOIN nodes n ON n.id=p.node_id LEFT JOIN ip_addresses a ON a.pool_id=p.id
		GROUP BY p.id, n.name ORDER BY p.id`)
	if err != nil {
		return err
	}
	type P struct {
		ID      int64  `json:"id"`
		Node    string `json:"node"`
		CIDR    string `json:"cidr"`
		Gateway string `json:"gateway"`
		DNS     string `json:"dns"`
		Total   int64  `json:"total"`
		Free    int64  `json:"free"`
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (P, error) {
		var p P
		return p, r.Scan(&p.ID, &p.Node, &p.CIDR, &p.Gateway, &p.DNS, &p.Total, &p.Free)
	})
	if err != nil {
		return err
	}
	return c.JSON(list)
}

// adminCreatePool CIDR-den IP salgylaryny döredýär (tor, broadcast we şlýuz salgylaryndan başga).
// Islege görä diňe [range_start, range_end] aralygy ulanylýar.
func (a *API) adminCreatePool(c *fiber.Ctx) error {
	var in struct {
		NodeID     int64    `json:"node_id"`
		CIDR       string   `json:"cidr"`
		Gateway    string   `json:"gateway"`
		DNS        string   `json:"dns"`
		RangeStart string   `json:"range_start"`
		RangeEnd   string   `json:"range_end"`
		Reserved   []string `json:"reserved"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	pfx, err := netip.ParsePrefix(trim(in.CIDR))
	if err != nil || !pfx.Addr().Is4() || pfx.Bits() < 16 || pfx.Bits() > 30 {
		return E(422, "invalid_cidr", "CIDR nädogry (IPv4, /16 – /30)")
	}
	pfx = pfx.Masked()
	gw, err := netip.ParseAddr(trim(in.Gateway))
	if err != nil || !pfx.Contains(gw) {
		return E(422, "invalid_gateway", "Şlýuz CIDR-iň içinde bolmaly")
	}
	lo, hi := pfx.Addr().Next(), lastAddr(pfx) // tor salgysyny we broadcast-y geçýäris
	if s, err := netip.ParseAddr(trim(in.RangeStart)); err == nil && pfx.Contains(s) && lo.Less(s) {
		lo = s
	}
	if e, err := netip.ParseAddr(trim(in.RangeEnd)); err == nil && pfx.Contains(e) && e.Less(hi) {
		hi = e.Next()
	}
	reserved := map[netip.Addr]bool{gw: true}
	for _, r := range in.Reserved {
		if ad, err := netip.ParseAddr(trim(r)); err == nil {
			reserved[ad] = true
		}
	}
	if in.DNS == "" {
		in.DNS = "8.8.8.8 1.1.1.1"
	}
	ctx := c.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var poolID int64
	if err := tx.QueryRow(ctx, `INSERT INTO ip_pools (node_id, cidr, gateway, dns) VALUES ($1,$2,$3,$4) RETURNING id`,
		in.NodeID, pfx.String(), gw.String(), trim(in.DNS)).Scan(&poolID); err != nil {
		return E(422, "invalid_node", "Node tapylmady")
	}
	n := 0
	for ad := lo; ad.Less(hi) && n < 4096; ad = ad.Next() {
		if reserved[ad] {
			continue
		}
		tag, err := tx.Exec(ctx, `INSERT INTO ip_addresses (pool_id, address, prefix) VALUES ($1,$2,$3) ON CONFLICT (address) DO NOTHING`,
			poolID, ad.String(), pfx.Bits())
		if err != nil {
			return err
		}
		n += int(tag.RowsAffected())
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	a.audit(ctx, c, "ip_pool_create", pfx.String(), map[string]any{"addresses": n})
	return c.Status(201).JSON(fiber.Map{"id": poolID, "addresses": n})
}

func lastAddr(p netip.Prefix) netip.Addr {
	b := p.Addr().As4()
	host := 32 - p.Bits()
	v := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	v |= (1 << host) - 1
	return netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
}

func (a *API) adminDeletePool(c *fiber.Ctx) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	var used int
	if err := a.DB.QueryRow(c.Context(), `SELECT count(*) FROM ip_addresses WHERE pool_id=$1 AND server_id IS NOT NULL`, id).Scan(&used); err != nil {
		return err
	}
	if used > 0 {
		return E(409, "pool_in_use", "Bu blokdaky IP-ler maşynlarda ulanylýar")
	}
	if _, err := a.DB.Exec(c.Context(), `DELETE FROM ip_pools WHERE id=$1`, id); err != nil {
		return err
	}
	a.audit(c.Context(), c, "ip_pool_delete", "", map[string]any{"pool_id": id})
	return c.JSON(fiber.Map{"ok": true})
}
