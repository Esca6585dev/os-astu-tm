// Package config sazlamalary gurşaw üýtgeýänlerinden (env) okaýar.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env         string // dev | prod
	HTTPAddr    string
	DatabaseURL string
	PublicURL   string // https://os.astu.tm — email salgylary üçin

	CookieSecure bool
	SessionTTL   time.Duration

	AdminEmail    string
	AdminPassword string

	// Hypervisor: "proxmox" ýa-da "fake" (ýerli synag üçin)
	Hypervisor      string
	PVEURL          string // https://10.0.0.10:8006
	PVETokenID      string // panel@pve!api
	PVETokenSecret  string
	PVEInsecureTLS  bool   // öz-özüne gol çekilen sertifikat
	PVECAFile       string // ýa-da CA faýly
	PVEDisk         string // şablondaky esasy disk: scsi0
	PVEMinCreditHrs int64  // maşyn açmak üçin gerek sagat sany
	PVEFirewall     bool   // her VM-e ipfilter + SMTP bloky

	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
	MailFrom string

	SuspendDeleteAfter time.Duration
	DevAutoVerify      bool
}

func Load() Config {
	return Config{
		Env:                get("APP_ENV", "dev"),
		HTTPAddr:           get("HTTP_ADDR", ":8080"),
		DatabaseURL:        get("DATABASE_URL", "postgres://vps:vps@localhost:5432/vps?sslmode=disable"),
		PublicURL:          strings.TrimRight(get("PUBLIC_URL", "http://localhost:3000"), "/"),
		CookieSecure:       getBool("COOKIE_SECURE", false),
		SessionTTL:         time.Duration(getInt("SESSION_DAYS", 30)) * 24 * time.Hour,
		AdminEmail:         get("ADMIN_EMAIL", ""),
		AdminPassword:      get("ADMIN_PASSWORD", ""),
		Hypervisor:         get("HYPERVISOR", "fake"),
		PVEURL:             strings.TrimRight(get("PVE_URL", ""), "/"),
		PVETokenID:         get("PVE_TOKEN_ID", ""),
		PVETokenSecret:     get("PVE_TOKEN_SECRET", ""),
		PVEInsecureTLS:     getBool("PVE_INSECURE_TLS", false),
		PVECAFile:          get("PVE_CA_FILE", ""),
		PVEDisk:            get("PVE_DISK", "scsi0"),
		PVEMinCreditHrs:    int64(getInt("MIN_CREDIT_HOURS", 24)),
		PVEFirewall:        getBool("PVE_FIREWALL", true),
		SMTPHost:           get("SMTP_HOST", ""),
		SMTPPort:           getInt("SMTP_PORT", 587),
		SMTPUser:           get("SMTP_USER", ""),
		SMTPPass:           get("SMTP_PASS", ""),
		MailFrom:           get("MAIL_FROM", "no-reply@os.astu.tm"),
		SuspendDeleteAfter: time.Duration(getInt("SUSPEND_DELETE_DAYS", 7)) * 24 * time.Hour,
		DevAutoVerify:      getBool("DEV_AUTO_VERIFY", false),
	}
}

func get(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func getInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

func getBool(k string, def bool) bool {
	if v, err := strconv.ParseBool(os.Getenv(k)); err == nil {
		return v
	}
	return def
}
