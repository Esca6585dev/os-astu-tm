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
}
