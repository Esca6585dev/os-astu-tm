// Package httpapi Fiber HTTP API-ni (/api/v1) gurýar.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Esca6585dev/os-astu-tm/api/internal/config"
	"github.com/Esca6585dev/os-astu-tm/api/internal/hv"
	"github.com/Esca6585dev/os-astu-tm/api/internal/mail"
	"github.com/Esca6585dev/os-astu-tm/api/internal/security"
)

const sessionCookie = "sid"

type API struct {
	Cfg  config.Config
	DB   *pgxpool.Pool
	HV   hv.Hypervisor
	Mail *mail.Mailer
}

type User struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	Balance  int64  `json:"balance"`
	Account  string `json:"account_no"`
	Verified bool   `json:"email_verified"`
}

func accountNo(id int64) string { return "U-" + strconv.FormatInt(10000+id, 10) }

// apiError ýalňyşlyklary bir görnüşde gaýtarýar: {"error":{"code","message"}}
type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e *apiError) Error() string { return e.Message }

func E(status int, code, msg string) error { return &apiError{status, code, msg} }

func errorHandler(c *fiber.Ctx, err error) error {
	var ae *apiError
	var fe *fiber.Error
	switch {
	case errors.As(err, &ae):
	case errors.As(err, &fe):
		ae = &apiError{fe.Code, "http_error", fe.Message}
	case errors.Is(err, pgx.ErrNoRows):
		ae = &apiError{404, "not_found", "Tapylmady"}
	default:
		slog.Error("unhandled", "path", c.Path(), "err", err)
		ae = &apiError{500, "internal", "Içki ýalňyşlyk"}
	}
	return c.Status(ae.Status).JSON(fiber.Map{"error": fiber.Map{"code": ae.Code, "message": ae.Message}})
}

func (a *API) App() *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler:          errorHandler,
		DisableStartupMessage: true,
		BodyLimit:             256 * 1024,
		ProxyHeader:           fiber.HeaderXForwardedFor,
		ReadTimeout:           30 * time.Second,
	})
	app.Use(recover.New())
	app.Use(requestLog)

	app.Get("/healthz", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()
		if err := a.DB.Ping(ctx); err != nil {
			return E(503, "db_down", "DB elýeterli däl")
		}
		return c.JSON(fiber.Map{"ok": true})
	})

	v1 := app.Group("/api/v1", csrfGuard, a.loadSession)

	authLimit := limiter.New(limiter.Config{Max: 10, Expiration: time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return E(429, "rate_limited", "Gaty köp synanyşyk. Bir minutdan soň gaýtalaň.")
		}})
	au := v1.Group("/auth")
	au.Post("/register", authLimit, a.register)
	au.Post("/verify", authLimit, a.verify)
	au.Post("/login", authLimit, a.login)
	au.Post("/logout", a.logout)
	au.Post("/forgot", authLimit, a.forgot)
	au.Post("/reset", authLimit, a.reset)

	v1.Get("/plans", a.listPlans)
	v1.Get("/images", a.listImages)

	me := v1.Group("", a.requireUser)
	me.Get("/me", a.getMe)
	me.Patch("/me", a.patchMe)
	me.Get("/me/balance", a.getBalance)
	me.Get("/me/transactions", a.listTransactions)
	me.Get("/ssh-keys", a.listKeys)
	me.Post("/ssh-keys", a.addKey)
	me.Delete("/ssh-keys/:id", a.deleteKey)

	createLimit := limiter.New(limiter.Config{Max: 10, Expiration: time.Hour,
		KeyGenerator: func(c *fiber.Ctx) string { return "u" + strconv.FormatInt(userOf(c).ID, 10) },
		LimitReached: func(c *fiber.Ctx) error {
			return E(429, "rate_limited", "Sagatda iň köp 10 maşyn döredip bolýar.")
		}})
	me.Get("/servers", a.listServers)
	me.Post("/servers", createLimit, a.createServer)
	me.Get("/servers/:id", a.getServer)
	me.Delete("/servers/:id", a.deleteServer)
	me.Post("/servers/:id/:action", a.serverAction)
	me.Get("/servers/:id/jobs", a.serverJobs)

	ad := v1.Group("/admin", a.requireUser, a.requireAdmin)
	ad.Get("/stats", a.adminStats)
	ad.Get("/users", a.adminUsers)
	ad.Patch("/users/:id", a.adminPatchUser)
	ad.Post("/users/:id/topup", a.adminTopup)
	ad.Get("/servers", a.adminServers)
	ad.Get("/transactions", a.adminTransactions)
	ad.Get("/audit", a.adminAudit)
	ad.Get("/plans", a.adminPlans)
	ad.Post("/plans", a.adminSavePlan)
	ad.Patch("/plans/:id", a.adminSavePlan)
	ad.Get("/images", a.adminImages)
	ad.Post("/images", a.adminSaveImage)
	ad.Patch("/images/:id", a.adminSaveImage)
	ad.Get("/nodes", a.adminNodes)
	ad.Post("/nodes", a.adminSaveNode)
	ad.Patch("/nodes/:id", a.adminSaveNode)
	ad.Get("/ip-pools", a.adminPools)
	ad.Post("/ip-pools", a.adminCreatePool)
	ad.Delete("/ip-pools/:id", a.adminDeletePool)

	return app
}

func requestLog(c *fiber.Ctx) error {
	start := time.Now()
	err := c.Next()
	status := c.Response().StatusCode()
	if err != nil {
		var ae *apiError
		if errors.As(err, &ae) {
			status = ae.Status
		}
	}
	slog.Info("http", "method", c.Method(), "path", c.Path(), "status", status, "ms", time.Since(start).Milliseconds())
	return err
}

// csrfGuard: üýtgeşme girizýän soraglarda X-Requested-With sözbaşysy gerek.
// Başga saýtlaryň formalary bu sözbaşyny goýup bilmeýär.
func csrfGuard(c *fiber.Ctx) error {
	switch c.Method() {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
		return c.Next()
	}
	if c.Get("X-Requested-With") == "" {
		return E(403, "csrf", "X-Requested-With sözbaşysy gerek")
	}
	return c.Next()
}

func (a *API) loadSession(c *fiber.Ctx) error {
	tok := c.Cookies(sessionCookie)
	if tok == "" {
		return c.Next()
	}
	var u User
	var verified *time.Time
	err := a.DB.QueryRow(c.Context(), `
		SELECT u.id, u.email, u.full_name, u.phone, u.role, u.status, u.balance, u.email_verified_at
		FROM sessions s JOIN users u ON u.id=s.user_id
		WHERE s.token_hash=$1 AND s.expires_at > now()`, security.HashToken(tok)).
		Scan(&u.ID, &u.Email, &u.FullName, &u.Phone, &u.Role, &u.Status, &u.Balance, &verified)
	if err == nil && u.Status != "blocked" {
		u.Account = accountNo(u.ID)
		u.Verified = verified != nil
		c.Locals("user", &u)
	}
	return c.Next()
}

func userOf(c *fiber.Ctx) *User {
	u, _ := c.Locals("user").(*User)
	return u
}

func (a *API) requireUser(c *fiber.Ctx) error {
	if userOf(c) == nil {
		return E(401, "unauthorized", "Giriş ediň")
	}
	return c.Next()
}

func (a *API) requireAdmin(c *fiber.Ctx) error {
	if userOf(c).Role != "admin" {
		return E(403, "forbidden", "Diňe administrator üçin")
	}
	return c.Next()
}

func (a *API) audit(ctx context.Context, c *fiber.Ctx, action, target string, meta map[string]any) {
	var actor *int64
	if u := userOf(c); u != nil {
		actor = &u.ID
	}
	if meta == nil {
		meta = map[string]any{}
	}
	if _, err := a.DB.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target, meta, ip) VALUES ($1,$2,$3,$4,$5)`,
		actor, action, target, meta, c.IP()); err != nil {
		slog.Error("audit", "err", err)
	}
}

func bind(c *fiber.Ctx, v any) error {
	if err := c.BodyParser(v); err != nil {
		return E(400, "bad_request", "Nädogry JSON")
	}
	return nil
}

func idParam(c *fiber.Ctx, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Params(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, E(404, "not_found", "Tapylmady")
	}
	return id, nil
}

func page(c *fiber.Ctx) (limit, offset int) {
	limit = c.QueryInt("limit", 25)
	if limit < 1 || limit > 100 {
		limit = 25
	}
	p := c.QueryInt("page", 1)
	if p < 1 {
		p = 1
	}
	return limit, (p - 1) * limit
}

func trim(s string) string { return strings.TrimSpace(s) }
