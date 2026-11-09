package hv

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Proxmox — Proxmox VE REST API (api2/json) klienti, API token bilen.
type Proxmox struct {
	base     string
	auth     string
	disk     string
	firewall bool
	client   *http.Client
}

type ProxmoxOptions struct {
	URL         string // https://pve.local:8006
	TokenID     string // panel@pve!api
	TokenSecret string
	InsecureTLS bool
	CAFile      string
	Disk        string // scsi0
	// Firewall: her maşyna IP/MAC filtri we çykýan SMTP (25) blokuny goýmak
	Firewall bool
}

func NewProxmox(o ProxmoxOptions) (*Proxmox, error) {
	if o.URL == "" || o.TokenID == "" || o.TokenSecret == "" {
		return nil, errors.New("PVE_URL, PVE_TOKEN_ID we PVE_TOKEN_SECRET gerek")
	}
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if o.CAFile != "" {
		pem, err := os.ReadFile(o.CAFile)
		if err != nil {
			return nil, fmt.Errorf("PVE_CA_FILE: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("PVE_CA_FILE: sertifikat okalmady")
		}
		tlsCfg.RootCAs = pool
	} else if o.InsecureTLS {
		tlsCfg.InsecureSkipVerify = true //nolint:gosec // diňe içki tor üçin, sazlamada aýdylýar
	}
	disk := o.Disk
	if disk == "" {
		disk = "scsi0"
	}
	return &Proxmox{
		base:     strings.TrimRight(o.URL, "/") + "/api2/json",
		auth:     fmt.Sprintf("PVEAPIToken=%s=%s", o.TokenID, o.TokenSecret),
		disk:     disk,
		firewall: o.Firewall,
		client: &http.Client{
			Timeout:   60 * time.Second,
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
		},
	}, nil
}

type pveError struct {
	Status int
	Body   string
}

func (e *pveError) Error() string { return fmt.Sprintf("proxmox %d: %s", e.Status, e.Body) }

func (p *Proxmox) do(ctx context.Context, method, path string, form url.Values, out any) error {
	var body io.Reader
	full := p.base + path
	if form != nil && (method == http.MethodGet || method == http.MethodDelete) {
		full += "?" + form.Encode()
	} else if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, full, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", p.auth)
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = resp.Status
		}
		return &pveError{Status: resp.StatusCode, Body: msg}
	}
	if out == nil {
		return nil
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("proxmox jogaby: %w", err)
	}
	return json.Unmarshal(env.Data, out)
}

// waitTask UPID task gutarýança garaşýar we netijesini barlaýar.
func (p *Proxmox) waitTask(ctx context.Context, node, upid string) error {
	if upid == "" {
		return nil
	}
	path := fmt.Sprintf("/nodes/%s/tasks/%s/status", url.PathEscape(node), url.PathEscape(upid))
	deadline := time.Now().Add(15 * time.Minute)
	for time.Now().Before(deadline) {
		var st struct {
			Status     string `json:"status"`
			ExitStatus string `json:"exitstatus"`
		}
		if err := p.do(ctx, http.MethodGet, path, nil, &st); err != nil {
			return err
		}
		if st.Status == "stopped" {
			if st.ExitStatus != "OK" && !strings.HasPrefix(st.ExitStatus, "WARNINGS") {
				return fmt.Errorf("proxmox task %s: %s", upid, st.ExitStatus)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1500 * time.Millisecond):
		}
	}
	return fmt.Errorf("proxmox task %s: wagt gutardy", upid)
}

// task UPID gaýtarýan haýyşy iberýär we tamamlanýança garaşýar.
func (p *Proxmox) task(ctx context.Context, node, method, path string, form url.Values) error {
	var upid any
	if err := p.do(ctx, method, path, form, &upid); err != nil {
		return err
	}
	s, _ := upid.(string)
	return p.waitTask(ctx, node, s)
}

func vmPath(node string, vmid int, suffix string) string {
	return fmt.Sprintf("/nodes/%s/qemu/%d%s", url.PathEscape(node), vmid, suffix)
}

func (p *Proxmox) NextVMID(ctx context.Context) (int, error) {
	var v any
	if err := p.do(ctx, http.MethodGet, "/cluster/nextid", nil, &v); err != nil {
		return 0, err
	}
	switch t := v.(type) {
	case string:
		return strconv.Atoi(t)
	case float64:
		return int(t), nil
	}
	return 0, fmt.Errorf("nextid: garaşylmadyk jogap %v", v)
}

// encodeSSHKeys: Proxmox sshkeys meýdanyny URL-encoded görnüşde garaşýar
// (boşluk %20 bolmaly, + bolmaly däl). Forma kodlamasy muny ýene bir gezek kodlaýar.
func encodeSSHKeys(keys []string) string {
	return strings.ReplaceAll(url.QueryEscape(strings.Join(keys, "\n")), "+", "%20")
}

// templateNode şablonyň haýsy node-da durandygyny tapýar (klasterde başga node-da bolup biler).
func (p *Proxmox) templateNode(ctx context.Context, vmid int, fallback string) (string, error) {
	var res []struct {
		VMID int    `json:"vmid"`
		Node string `json:"node"`
	}
	if err := p.do(ctx, http.MethodGet, "/cluster/resources", url.Values{"type": {"vm"}}, &res); err != nil {
		return "", err
	}
	for _, r := range res {
		if r.VMID == vmid {
			return r.Node, nil
		}
	}
	return fallback, fmt.Errorf("şablon VM %d tapylmady", vmid)
}

func (p *Proxmox) Create(ctx context.Context, s VMSpec) error {
	// 1) Şablony doly klonlamak. Şablon başga node-da bolsa, ol paýlaşylýan
	// storage-da bolmaly (Proxmox şerti); klon şablonyň node-unda başlanýar.
	tplNode, err := p.templateNode(ctx, s.TemplateVMID, s.Node)
	if err != nil {
		return fmt.Errorf("clone: %w", err)
	}
	clone := url.Values{
		"newid":   {strconv.Itoa(s.VMID)},
		"name":    {s.Name},
		"full":    {"1"},
		"storage": {s.Storage},
	}
	if tplNode != s.Node {
		clone.Set("target", s.Node)
	}
	if err := p.task(ctx, tplNode, http.MethodPost, vmPath(tplNode, s.TemplateVMID, "/clone"), clone); err != nil {
		return fmt.Errorf("clone: %w", err)
	}

	// 2) CPU, RAM, tor tizligi, cloud-init
	rateMBs := strconv.FormatFloat(float64(s.BandwidthMbit)/8, 'f', 3, 64)
	cfg := url.Values{
		"cores":     {strconv.Itoa(s.Cores)},
		"memory":    {strconv.Itoa(s.MemoryMB)},
		"net0":      {fmt.Sprintf("virtio,bridge=%s,firewall=1,rate=%s", s.Bridge, rateMBs)},
		"ipconfig0": {fmt.Sprintf("ip=%s/%d,gw=%s", s.IP, s.Prefix, s.Gateway)},
		"ciuser":    {"root"},
		"agent":     {"1"},
		"onboot":    {"1"},
	}
	if s.DNS != "" {
		cfg.Set("nameserver", s.DNS)
	}
	if s.Password != "" {
		cfg.Set("cipassword", s.Password)
	}
	if len(s.SSHKeys) > 0 {
		cfg.Set("sshkeys", encodeSSHKeys(s.SSHKeys))
	}
	if err := p.do(ctx, http.MethodPut, vmPath(s.Node, s.VMID, "/config"), cfg, nil); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if p.firewall {
		if err := p.applyFirewall(ctx, s); err != nil {
			return fmt.Errorf("firewall: %w", err)
		}
	}

	// 3) Diski tarife çenli ulaltmak (absolýut ölçeg)
	resize := url.Values{"disk": {p.disk}, "size": {fmt.Sprintf("%dG", s.DiskGB)}}
	if err := p.task(ctx, s.Node, http.MethodPut, vmPath(s.Node, s.VMID, "/resize"), resize); err != nil {
		var pe *pveError
		// Şablonyň diski eýýäm şu ölçegde bolsa Proxmox ýalňyşlyk berýär — muny geçýäris
		if !(errors.As(err, &pe) && strings.Contains(pe.Body, "shrinking disks is not supported")) {
			return fmt.Errorf("resize: %w", err)
		}
	}

	// 4) Başlatmak
	if err := p.Start(ctx, s.Node, s.VMID); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	return nil
}

// applyFirewall: maşyn diňe öz IP-sini ulanyp biler (ipfilter), MAC çalşyp bilmez,
// çykýan SMTP (25) ýapyk — spam we IP ogurlygyna garşy. Datacenter firewall açyk bolmaly.
func (p *Proxmox) applyFirewall(ctx context.Context, s VMSpec) error {
	base := vmPath(s.Node, s.VMID, "/firewall")
	if err := p.do(ctx, http.MethodPut, base+"/options", url.Values{
		"enable": {"1"}, "ipfilter": {"1"}, "macfilter": {"1"}, "dhcp": {"0"}, "ndp": {"0"}, "radv": {"0"},
		"policy_in": {"ACCEPT"}, "policy_out": {"ACCEPT"},
	}, nil); err != nil {
		return err
	}
	if err := p.do(ctx, http.MethodPost, base+"/ipset", url.Values{"name": {"ipfilter-net0"}}, nil); err != nil && !strings.Contains(err.Error(), "already exist") {
		return err
	}
	if err := p.do(ctx, http.MethodPost, base+"/ipset/ipfilter-net0", url.Values{"cidr": {s.IP}}, nil); err != nil && !strings.Contains(err.Error(), "already exist") {
		return err
	}
	return p.do(ctx, http.MethodPost, base+"/rules", url.Values{
		"type": {"out"}, "action": {"DROP"}, "proto": {"tcp"}, "dport": {"25"}, "enable": {"1"}, "comment": {"SMTP ýapyk (panel)"},
	}, nil)
}

func (p *Proxmox) Start(ctx context.Context, node string, vmid int) error {
	return p.task(ctx, node, http.MethodPost, vmPath(node, vmid, "/status/start"), url.Values{})
}

func (p *Proxmox) Stop(ctx context.Context, node string, vmid int) error {
	// Ilki ýumşak öçürmek, 60 sekuntdan soň zor bilen
	err := p.task(ctx, node, http.MethodPost, vmPath(node, vmid, "/status/shutdown"),
		url.Values{"timeout": {"60"}, "forceStop": {"1"}})
	if err != nil && isNotRunning(err) {
		return nil
	}
	return err
}

func (p *Proxmox) Reboot(ctx context.Context, node string, vmid int) error {
	return p.task(ctx, node, http.MethodPost, vmPath(node, vmid, "/status/reboot"), url.Values{"timeout": {"60"}})
}

func (p *Proxmox) SetPassword(ctx context.Context, node string, vmid int, user, password string) error {
	form := url.Values{"username": {user}, "password": {password}}
	if err := p.do(ctx, http.MethodPost, vmPath(node, vmid, "/agent/set-user-password"), form, nil); err != nil {
		return fmt.Errorf("qemu-guest-agent: %w", err)
	}
	// Indiki täzeden gurlanda hem şol parol bolar ýaly cloud-init-de hem saklaýarys
	return p.do(ctx, http.MethodPut, vmPath(node, vmid, "/config"), url.Values{"cipassword": {password}}, nil)
}

func (p *Proxmox) Destroy(ctx context.Context, node string, vmid int) error {
	if err := p.task(ctx, node, http.MethodPost, vmPath(node, vmid, "/status/stop"), url.Values{}); err != nil {
		if isMissing(err) {
			return nil
		}
		if !isNotRunning(err) {
			return fmt.Errorf("stop: %w", err)
		}
	}
	err := p.task(ctx, node, http.MethodDelete, vmPath(node, vmid, ""),
		url.Values{"purge": {"1"}, "destroy-unreferenced-disks": {"1"}})
	if err != nil && isMissing(err) {
		return nil
	}
	return err
}

func (p *Proxmox) Usage(ctx context.Context, node string, vmid int) (Usage, error) {
	var st struct {
		Status  string  `json:"status"`
		CPU     float64 `json:"cpu"`
		Mem     int64   `json:"mem"`
		MaxMem  int64   `json:"maxmem"`
		MaxDisk int64   `json:"maxdisk"`
		NetIn   int64   `json:"netin"`
		NetOut  int64   `json:"netout"`
		Uptime  int64   `json:"uptime"`
	}
	if err := p.do(ctx, http.MethodGet, vmPath(node, vmid, "/status/current"), nil, &st); err != nil {
		if isMissing(err) {
			return Usage{}, ErrNotFound
		}
		return Usage{}, err
	}
	return Usage{Status: st.Status, CPU: st.CPU, MemUsed: st.Mem, MemMax: st.MaxMem, DiskMax: st.MaxDisk,
		NetInBytes: st.NetIn, NetOutBytes: st.NetOut, UptimeSec: st.Uptime}, nil
}

func isMissing(err error) bool {
	var pe *pveError
	return errors.As(err, &pe) && (strings.Contains(pe.Body, "does not exist") || pe.Status == http.StatusNotFound)
}

func isNotRunning(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not running")
}
