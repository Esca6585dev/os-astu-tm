// Package hv wirtualizasiýa platformasy bilen işleýär. Proxmox adapteri we
// ýerli synag üçin fake adapteri bir interfeýsi kanagatlandyrýar.
package hv

import (
	"context"
	"errors"
)

// VMSpec — täze maşyn üçin gerek ähli maglumat.
type VMSpec struct {
	Node          string
	VMID          int
	Name          string
	TemplateVMID  int
	Storage       string
	Bridge        string
	Cores         int
	MemoryMB      int
	DiskGB        int
	BandwidthMbit int
	IP            string // 203.0.113.24
	Prefix        int    // 24
	Gateway       string
	DNS           string // "8.8.8.8 1.1.1.1"
	Password      string // boş bolup biler
	SSHKeys       []string
	Firewall      Firewall
}

// FWRule — ulanyjynyň firewall düzgüni (Proxmox VM firewall).
type FWRule struct {
	Direction string `json:"direction"` // in | out
	Action    string `json:"action"`    // ACCEPT | DROP
	Proto     string `json:"proto"`     // "" (hemmesi) | tcp | udp | icmp
	Dport     string `json:"dport"`     // "22", "80,443", "8000:8100"
	Addr      string `json:"addr"`      // in: çeşme, out: barýan ýeri (IP ýa-da CIDR), boş = islendik
	Comment   string `json:"comment"`
}

// Firewall — maşynyň doly firewall sazlamasy.
type Firewall struct {
	PolicyIn string   // ACCEPT | DROP: hiç bir düzgüne gabat gelmeýän gelýän trafik
	Rules    []FWRule // ýokardan aşak barlanýar
}

// Custom ulanyjy firewall-y üýtgedendigini görkezýär.
func (f Firewall) Custom() bool { return f.PolicyIn == "DROP" || len(f.Rules) > 0 }

// SMTPBlock panel her maşyna goýýan we ulanyjynyň aýryp bilmeýän düzgüni (spam garşy).
var SMTPBlock = FWRule{Direction: "out", Action: "DROP", Proto: "tcp", Dport: "25", Comment: "SMTP ýapyk (panel)"}

type BackupInfo struct {
	Volid string // storage:backup/vzdump-qemu-...
	Size  int64
}

type Usage struct {
	Status      string  `json:"status"` // running | stopped
	CPU         float64 `json:"cpu"`    // 0..1
	MemUsed     int64   `json:"mem_used"`
	MemMax      int64   `json:"mem_max"`
	DiskMax     int64   `json:"disk_max"`
	NetInBytes  int64   `json:"net_in"`
	NetOutBytes int64   `json:"net_out"`
	UptimeSec   int64   `json:"uptime"`
}

var ErrNotFound = errors.New("vm not found")

type Hypervisor interface {
	// NextVMID boş VMID gaýtarýar.
	NextVMID(ctx context.Context) (int, error)
	// Create şablony klonlaýar, sazlaýar, diski ulaldýar we başladýar.
	Create(ctx context.Context, s VMSpec) error
	Start(ctx context.Context, node string, vmid int) error
	Stop(ctx context.Context, node string, vmid int) error
	Reboot(ctx context.Context, node string, vmid int) error
	// SetPassword root parolyny qemu-guest-agent arkaly çalyşýar.
	SetPassword(ctx context.Context, node string, vmid int, user, password string) error
	// Destroy maşyny durdurýar we diskleri bilen pozýar.
	Destroy(ctx context.Context, node string, vmid int) error
	Usage(ctx context.Context, node string, vmid int) (Usage, error)
	// SetFirewall maşynyň firewall düzgünlerini we syýasatyny doly çalyşýar.
	SetFirewall(ctx context.Context, node string, vmid int, ip string, fw Firewall) error
	// Backup maşynyň doly bekabyny (vzdump) storage-a ýazýar. tag bekaba bellik hökmünde goşulýar.
	Backup(ctx context.Context, node string, vmid int, storage, tag string) (BackupInfo, error)
	// RestoreBackup maşyny durdurýar we ony bekapdan dikeldýär. Maşyn durdurylan galýar.
	RestoreBackup(ctx context.Context, node string, vmid int, volid, storage string) error
	// DeleteBackup bekap faýlyny pozýar (ýok bolsa ýalňyşlyk däl).
	DeleteBackup(ctx context.Context, node, volid string) error
}
