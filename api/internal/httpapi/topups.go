package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"github.com/Esca6585dev/os-astu-tm/api/internal/billing"
)

// Balans doldurmak: ulanyjy tölegi bank, kassa ýa-da kart arkaly geçirýär we panelde
// haýyş iberýär. Administrator tölegi barlap tassyklaýar we balans şol pursat goşulýar.

var topupMethods = map[string]string{
	"bank": "Bank geçirimi",
	"cash": "Kassa (nagt)",
	"card": "Bank kartyndan",
}

const maxPendingTopups = 5

type Topup struct {
	ID          int64      `json:"id"`
	Amount      int64      `json:"amount"`
	Method      string     `json:"method"`
	Reference   string     `json:"reference"`
	Comment     string     `json:"comment"`
	Status      string     `json:"status"`
	AdminNote   string     `json:"admin_note"`
	CreatedAt   time.Time  `json:"created_at"`
	DecidedAt   *time.Time `json:"decided_at"`
	UserID      int64      `json:"user_id,omitempty"`
	UserEmail   string     `json:"user_email,omitempty"`
	FullName    string     `json:"full_name,omitempty"`
	AccountNo   string     `json:"account_no,omitempty"`
	UserBalance *int64     `json:"user_balance,omitempty"`
}

const topupSelect = `
	SELECT t.id, t.amount, t.method, t.reference, t.comment, t.status, t.admin_note, t.created_at, t.decided_at,
	       u.id, u.email, u.full_name, u.balance
	FROM topup_requests t JOIN users u ON u.id=t.user_id`

func collectTopups(rows pgx.Rows, admin bool) ([]Topup, error) {
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Topup, error) {
		var t Topup
		var bal int64
		err := r.Scan(&t.ID, &t.Amount, &t.Method, &t.Reference, &t.Comment, &t.Status, &t.AdminNote, &t.CreatedAt, &t.DecidedAt,
			&t.UserID, &t.UserEmail, &t.FullName, &bal)
		if admin {
			t.AccountNo = accountNo(t.UserID)
			t.UserBalance = &bal
		} else {
			t.UserID, t.UserEmail, t.FullName = 0, "", ""
		}
		return t, err
	})
}

// credit balansy üýtgedýär we amallar taryhyna ýazýar (tranzaksiýanyň içinde).
func credit(ctx context.Context, tx pgx.Tx, userID, tenge int64, typ string, adminID int64, note string) (balance, txID int64, err error) {
	err = tx.QueryRow(ctx, `UPDATE users SET balance=balance+$2, low_balance_notified_at=NULL WHERE id=$1 RETURNING balance`, userID, tenge).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, E(404, "not_found", "Ulanyjy tapylmady")
	} else if err != nil {
		return 0, 0, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO transactions (user_id, type, amount, balance_after, admin_id, note) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		userID, typ, tenge, balance, adminID, note).Scan(&txID)
	return balance, txID, err
}

// mailAsync haty fonda iberýär: SMTP haýal bolsa-da API jogaby garaşmaýar.
func (a *API) mailAsync(to, subject, body string) {
	go func() {
		if err := a.Mail.Send(to, subject, body); err != nil {
			slog.Error("mail", "to", to, "err", err)
		}
	}()
}

func (a *API) listTopups(c *fiber.Ctx) error {
	rows, err := a.DB.Query(c.Context(), topupSelect+` WHERE t.user_id=$1 ORDER BY t.id DESC LIMIT 20`, userOf(c).ID)
	if err != nil {
		return err
	}
	list, err := collectTopups(rows, false)
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) createTopup(c *fiber.Ctx) error {
	u := userOf(c)
	var in struct {
		Amount    float64 `json:"amount"` // TMT
		Method    string  `json:"method"`
		Reference string  `json:"reference"`
		Comment   string  `json:"comment"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	tenge := int64(math.Round(in.Amount * 100))
	if tenge < 100 || tenge > 10_000_000 {
		return E(422, "invalid_amount", "Möçber 1 – 100 000 TMT aralygynda bolmaly")
	}
	method, ok := topupMethods[in.Method]
	if !ok {
		return E(422, "invalid_method", "Töleg usulyny saýlaň")
	}
	ref := cleanText(in.Reference, 80)
	if ref == "" {
		return E(422, "reference_required", "Çekiň ýa-da tölegiň belgisini ýazyň")
	}
	comment := cleanText(in.Comment, 200)

	ctx := c.Context()
	var pending int
	if err := a.DB.QueryRow(ctx, `SELECT count(*) FROM topup_requests WHERE user_id=$1 AND status='pending'`, u.ID).Scan(&pending); err != nil {
		return err
	}
	if pending >= maxPendingTopups {
		return E(409, "too_many_pending", fmt.Sprintf("Size %d haýyş garaşylýar. Olara seredilmegine garaşyň.", pending))
	}
	var id int64
	if err := a.DB.QueryRow(ctx, `INSERT INTO topup_requests (user_id, amount, method, reference, comment) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		u.ID, tenge, in.Method, ref, comment).Scan(&id); err != nil {
		return err
	}
	a.audit(ctx, c, "topup_request", u.Account, map[string]any{"request_id": id, "amount": tenge, "method": in.Method})

	// Administratorlara habar
	rows, err := a.DB.Query(ctx, `SELECT email FROM users WHERE role='admin' AND status='active'`)
	if err == nil {
		admins, _ := pgx.CollectRows(rows, pgx.RowTo[string])
		body := fmt.Sprintf("%s (%s) balansy doldurmak üçin haýyş iberdi.\n\nMöçber: %s TMT\nUsul: %s\nÇek / töleg belgisi: %s\n%s\nTassyklamak: %s/admin",
			u.Account, u.Email, billing.FormatTMT(tenge), method, ref, comment, a.Cfg.PublicURL)
		for _, to := range admins {
			a.mailAsync(to, fmt.Sprintf("Doldurma haýyşy: %s · %s TMT", u.Account, billing.FormatTMT(tenge)), body)
		}
	}
	return c.Status(201).JSON(fiber.Map{"id": id})
}

func (a *API) cancelTopup(c *fiber.Ctx) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	var status string
	err = a.DB.QueryRow(c.Context(), `SELECT status FROM topup_requests WHERE id=$1 AND user_id=$2`, id, userOf(c).ID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return E(404, "not_found", "Haýyş tapylmady")
	} else if err != nil {
		return err
	}
	tag, err := a.DB.Exec(c.Context(), `UPDATE topup_requests SET status='cancelled', decided_at=now() WHERE id=$1 AND status='pending'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return E(409, "already_decided", "Bu haýyşa eýýäm seredildi, ony ýatyryp bolmaýar")
	}
	a.audit(c.Context(), c, "topup_cancel", userOf(c).Account, map[string]any{"request_id": id})
	return c.JSON(fiber.Map{"ok": true})
}

// ---------- Admin ----------

func (a *API) adminTopups(c *fiber.Ctx) error {
	limit, offset := page(c)
	status := c.Query("status", "pending")
	if status == "all" {
		status = ""
	}
	rows, err := a.DB.Query(c.Context(), topupSelect+`
		WHERE ($1 = '' OR t.status=$1)
		ORDER BY (t.status='pending') DESC, t.id DESC LIMIT $2 OFFSET $3`, status, limit, offset)
	if err != nil {
		return err
	}
	list, err := collectTopups(rows, true)
	if err != nil {
		return err
	}
	return c.JSON(list)
}

type pendingTopup struct {
	UserID    int64
	Amount    int64
	Method    string
	Reference string
	Status    string
	Email     string
}

// lockTopup haýyşy tranzaksiýanyň içinde gulplaýar we diňe garaşylýan bolsa gaýtarýar.
func lockTopup(ctx context.Context, tx pgx.Tx, id int64) (pendingTopup, error) {
	var t pendingTopup
	err := tx.QueryRow(ctx, `
		SELECT t.user_id, t.amount, t.method, t.reference, t.status, u.email
		FROM topup_requests t JOIN users u ON u.id=t.user_id
		WHERE t.id=$1 FOR UPDATE OF t`, id).Scan(&t.UserID, &t.Amount, &t.Method, &t.Reference, &t.Status, &t.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, E(404, "not_found", "Haýyş tapylmady")
	} else if err != nil {
		return t, err
	}
	if t.Status != "pending" {
		return t, E(409, "already_decided", "Bu haýyşa eýýäm seredildi")
	}
	return t, nil
}

func (a *API) adminApproveTopup(c *fiber.Ctx) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Note string `json:"note"`
	}
	if len(c.Body()) > 0 {
		if err := bind(c, &in); err != nil {
			return err
		}
	}
	adminNote := cleanText(in.Note, 200)
	ctx := c.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	t, err := lockTopup(ctx, tx, id)
	if err != nil {
		return err
	}
	note := fmt.Sprintf("Haýyş #%d · %s · %s", id, topupMethods[t.Method], t.Reference)
	bal, txID, err := credit(ctx, tx, t.UserID, t.Amount, "topup", userOf(c).ID, note)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE topup_requests SET status='approved', admin_id=$2, admin_note=$3, transaction_id=$4, decided_at=now() WHERE id=$1`,
		id, userOf(c).ID, adminNote, txID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	a.audit(ctx, c, "topup_approve", accountNo(t.UserID), map[string]any{"request_id": id, "amount": t.Amount})
	a.mailAsync(t.Email, "Balansyňyz dolduryldy: +"+billing.FormatTMT(t.Amount)+" TMT",
		fmt.Sprintf("Doldurma haýyşyňyz (#%d) tassyklandy.\nGoşuldy: %s TMT\nTäze balans: %s TMT\n\nÖçürilen serwerleriňiz bir minudyň dowamynda awtomatik açylar.\n\n%s/billing",
			id, billing.FormatTMT(t.Amount), billing.FormatTMT(bal), a.Cfg.PublicURL))
	return c.JSON(fiber.Map{"balance": bal})
}

func (a *API) adminRejectTopup(c *fiber.Ctx) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Note string `json:"note"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	reason := cleanText(in.Note, 200)
	if reason == "" {
		return E(422, "note_required", "Ret etmegiň sebäbini ýazyň (ulanyjy görer)")
	}
	ctx := c.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	t, err := lockTopup(ctx, tx, id)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE topup_requests SET status='rejected', admin_id=$2, admin_note=$3, decided_at=now() WHERE id=$1`,
		id, userOf(c).ID, reason); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	a.audit(ctx, c, "topup_reject", accountNo(t.UserID), map[string]any{"request_id": id, "amount": t.Amount, "reason": reason})
	a.mailAsync(t.Email, "Doldurma haýyşyňyz ret edildi",
		fmt.Sprintf("Doldurma haýyşyňyz (#%d, %s TMT) ret edildi.\nSebäbi: %s\n\n%s/billing", id, billing.FormatTMT(t.Amount), reason, a.Cfg.PublicURL))
	return c.JSON(fiber.Map{"ok": true})
}
