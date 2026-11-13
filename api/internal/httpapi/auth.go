package httpapi

import (
	"context"
	"fmt"
	"net/mail"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"github.com/Esca6585dev/os-astu-tm/api/internal/security"
)

func (a *API) setSession(c *fiber.Ctx, userID int64) error {
	tok, hash := security.NewToken()
	exp := time.Now().Add(a.Cfg.SessionTTL)
	if _, err := a.DB.Exec(c.Context(), `INSERT INTO sessions (user_id, token_hash, ip, user_agent, expires_at) VALUES ($1,$2,$3,$4,$5)`,
		userID, hash, c.IP(), truncate(c.Get("User-Agent"), 300), exp); err != nil {
		return err
	}
	c.Cookie(&fiber.Cookie{Name: sessionCookie, Value: tok, Path: "/", Expires: exp,
		HTTPOnly: true, Secure: a.Cfg.CookieSecure, SameSite: fiber.CookieSameSiteLaxMode})
	return nil
}

// truncate setiri iň köp n baýta çenli gysgaldýar, köp baýtly harpy (ň, ş, ä…) ortasyndan bölmeýär.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

func validEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	return err == nil && addr.Address == s && len(s) <= 200
}

func (a *API) emailToken(ctx context.Context, userID int64, purpose string, ttl time.Duration) (string, error) {
	tok, hash := security.NewToken()
	_, err := a.DB.Exec(ctx, `INSERT INTO email_tokens (user_id, purpose, token_hash, expires_at) VALUES ($1,$2,$3,$4)`,
		userID, purpose, hash, time.Now().Add(ttl))
	return tok, err
}

func (a *API) register(c *fiber.Ctx) error {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		FullName string `json:"full_name"`
		Phone    string `json:"phone"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	in.Email = trim(in.Email)
	if !validEmail(in.Email) {
		return E(422, "invalid_email", "Email nädogry")
	}
	if len(in.Password) < 10 || len(in.Password) > 200 {
		return E(422, "weak_password", "Parol azyndan 10 simwol bolmaly")
	}
	hash, err := security.HashPassword(in.Password)
	if err != nil {
		return err
	}
	status, verifiedSQL := "pending", "NULL"
	if a.Cfg.DevAutoVerify {
		status, verifiedSQL = "active", "now()"
	}
	var id int64
	err = a.DB.QueryRow(c.Context(), `INSERT INTO users (email, password_hash, full_name, phone, status, email_verified_at)
		VALUES ($1,$2,$3,$4,$5,`+verifiedSQL+`) ON CONFLICT (email) DO NOTHING RETURNING id`,
		in.Email, hash, truncate(trim(in.FullName), 120), truncate(trim(in.Phone), 30), status).Scan(&id)
	if err == pgx.ErrNoRows {
		return E(409, "email_taken", "Bu email bilen hasap eýýäm bar")
	}
	if err != nil {
		return err
	}
	if !a.Cfg.DevAutoVerify {
		tok, err := a.emailToken(c.Context(), id, "verify", 48*time.Hour)
		if err != nil {
			return err
		}
		_ = a.Mail.Send(in.Email, "Emailiňizi tassyklaň",
			fmt.Sprintf("Salam!\n\nHasabyňyzy tassyklamak üçin şu salga geçiň:\n%s/verify?token=%s\n\nSalgy 48 sagat işleýär.", a.Cfg.PublicURL, tok))
	}
	return c.Status(201).JSON(fiber.Map{"ok": true, "verify_required": !a.Cfg.DevAutoVerify})
}

func (a *API) useToken(ctx context.Context, tok, purpose string) (int64, error) {
	var uid int64
	err := a.DB.QueryRow(ctx, `UPDATE email_tokens SET used_at=now()
		WHERE token_hash=$1 AND purpose=$2 AND used_at IS NULL AND expires_at > now() RETURNING user_id`,
		security.HashToken(tok), purpose).Scan(&uid)
	if err == pgx.ErrNoRows {
		return 0, E(400, "invalid_token", "Salgy nädogry ýa-da möhleti geçen")
	}
	return uid, err
}

func (a *API) verify(c *fiber.Ctx) error {
	var in struct {
		Token string `json:"token"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	uid, err := a.useToken(c.Context(), in.Token, "verify")
	if err != nil {
		return err
	}
	if _, err := a.DB.Exec(c.Context(), `UPDATE users SET email_verified_at=now(), status=CASE WHEN status='pending' THEN 'active' ELSE status END WHERE id=$1`, uid); err != nil {
		return err
	}
	if err := a.setSession(c, uid); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) login(c *fiber.Ctx) error {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	var id int64
	var hash, status string
	var verified *time.Time
	err := a.DB.QueryRow(c.Context(), `SELECT id, password_hash, status, email_verified_at FROM users WHERE email=$1`, trim(in.Email)).
		Scan(&id, &hash, &status, &verified)
	if err == pgx.ErrNoRows {
		security.CheckPassword("$argon2id$v=19$m=65536,t=2,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", in.Password)
		return E(401, "bad_credentials", "Email ýa-da parol nädogry")
	}
	if err != nil {
		return err
	}
	if !security.CheckPassword(hash, in.Password) {
		a.audit(c.Context(), c, "login_failed", in.Email, nil)
		return E(401, "bad_credentials", "Email ýa-da parol nädogry")
	}
	if status == "blocked" {
		return E(403, "blocked", "Hasabyňyz bloklanypdyr. Administrator bilen habarlaşyň.")
	}
	if verified == nil {
		return E(403, "email_not_verified", "Ilki emailiňizi tassyklaň")
	}
	if err := a.setSession(c, id); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) logout(c *fiber.Ctx) error {
	if tok := c.Cookies(sessionCookie); tok != "" {
		_, _ = a.DB.Exec(c.Context(), `DELETE FROM sessions WHERE token_hash=$1`, security.HashToken(tok))
	}
	c.Cookie(&fiber.Cookie{Name: sessionCookie, Value: "", Path: "/", Expires: time.Unix(0, 0), HTTPOnly: true,
		Secure: a.Cfg.CookieSecure, SameSite: fiber.CookieSameSiteLaxMode})
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) forgot(c *fiber.Ctx) error {
	var in struct {
		Email string `json:"email"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	var id int64
	if err := a.DB.QueryRow(c.Context(), `SELECT id FROM users WHERE email=$1 AND status <> 'blocked'`, trim(in.Email)).Scan(&id); err == nil {
		if tok, err := a.emailToken(c.Context(), id, "reset", time.Hour); err == nil {
			_ = a.Mail.Send(trim(in.Email), "Paroly dikeltmek",
				fmt.Sprintf("Täze parol goýmak üçin şu salga geçiň:\n%s/reset?token=%s\n\nSalgy 1 sagat işleýär. Eger siz soramadyk bolsaňyz, bu haty äsgermäň.", a.Cfg.PublicURL, tok))
		}
	}
	// Email barmy-ýokmy aýdýan däldiris
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) reset(c *fiber.Ctx) error {
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	if len(in.Password) < 10 || len(in.Password) > 200 {
		return E(422, "weak_password", "Parol azyndan 10 simwol bolmaly")
	}
	uid, err := a.useToken(c.Context(), in.Token, "reset")
	if err != nil {
		return err
	}
	hash, err := security.HashPassword(in.Password)
	if err != nil {
		return err
	}
	if _, err := a.DB.Exec(c.Context(), `UPDATE users SET password_hash=$2, email_verified_at=COALESCE(email_verified_at, now()),
		status=CASE WHEN status='pending' THEN 'active' ELSE status END WHERE id=$1`, uid, hash); err != nil {
		return err
	}
	_, _ = a.DB.Exec(c.Context(), `DELETE FROM sessions WHERE user_id=$1`, uid)
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) getMe(c *fiber.Ctx) error { return c.JSON(userOf(c)) }

func (a *API) patchMe(c *fiber.Ctx) error {
	u := userOf(c)
	var in struct {
		FullName        *string `json:"full_name"`
		Phone           *string `json:"phone"`
		CurrentPassword string  `json:"current_password"`
		NewPassword     string  `json:"new_password"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	if in.FullName != nil || in.Phone != nil {
		if _, err := a.DB.Exec(c.Context(), `UPDATE users SET full_name=COALESCE($2,full_name), phone=COALESCE($3,phone) WHERE id=$1`,
			u.ID, in.FullName, in.Phone); err != nil {
			return err
		}
	}
	if in.NewPassword != "" {
		if len(in.NewPassword) < 10 {
			return E(422, "weak_password", "Parol azyndan 10 simwol bolmaly")
		}
		var hash string
		if err := a.DB.QueryRow(c.Context(), `SELECT password_hash FROM users WHERE id=$1`, u.ID).Scan(&hash); err != nil {
			return err
		}
		if !security.CheckPassword(hash, in.CurrentPassword) {
			return E(422, "bad_password", "Häzirki parol nädogry")
		}
		nh, err := security.HashPassword(in.NewPassword)
		if err != nil {
			return err
		}
		if _, err := a.DB.Exec(c.Context(), `UPDATE users SET password_hash=$2 WHERE id=$1`, u.ID, nh); err != nil {
			return err
		}
		_, _ = a.DB.Exec(c.Context(), `DELETE FROM sessions WHERE user_id=$1 AND token_hash<>$2`, u.ID, security.HashToken(c.Cookies(sessionCookie)))
	}
	return c.JSON(fiber.Map{"ok": true})
}

// EnsureAdmin ADMIN_EMAIL/ADMIN_PASSWORD berlen bolsa, ilkinji administratory döredýär.
func (a *API) EnsureAdmin(ctx context.Context) error {
	if a.Cfg.AdminEmail == "" || a.Cfg.AdminPassword == "" {
		return nil
	}
	hash, err := security.HashPassword(a.Cfg.AdminPassword)
	if err != nil {
		return err
	}
	_, err = a.DB.Exec(ctx, `INSERT INTO users (email, password_hash, full_name, role, status, email_verified_at)
		VALUES ($1,$2,'Administrator','admin','active',now()) ON CONFLICT (email) DO UPDATE SET role='admin'`, a.Cfg.AdminEmail, hash)
	return err
}
