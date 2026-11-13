package hv

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// Proxmox API-ni simulýasiýa edip, Create-iň dogry haýyşlary iberýändigini barlaýarys.
func TestProxmoxCreate(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	var configForm url.Values
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "PVEAPIToken=panel@pve!api=secret" {
			w.WriteHeader(401)
			return
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/qemu/1001/config") {
			configForm, _ = url.ParseQuery(string(body))
		}
		mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/cluster/resources"):
			io.WriteString(w, `{"data":[{"vmid":9000,"node":"pve1"},{"vmid":5,"node":"pve2"}]}`)
		case strings.Contains(r.URL.Path, "/firewall"):
			io.WriteString(w, `{"data":null}`)
		case strings.Contains(r.URL.Path, "/tasks/"):
			io.WriteString(w, `{"data":{"status":"stopped","exitstatus":"OK"}}`)
		case strings.HasSuffix(r.URL.Path, "/config"):
			io.WriteString(w, `{"data":null}`)
		default:
			io.WriteString(w, `{"data":"UPID:pve1:0001:task:"}`)
		}
	}))
	defer srv.Close()

	p, err := NewProxmox(ProxmoxOptions{URL: srv.URL, TokenID: "panel@pve!api", TokenSecret: "secret", InsecureTLS: true, Firewall: true})
	if err != nil {
		t.Fatal(err)
	}
	err = p.Create(context.Background(), VMSpec{
		Node: "pve1", VMID: 1001, Name: "web-01", TemplateVMID: 9000, Storage: "local-lvm", Bridge: "vmbr0",
		Cores: 2, MemoryMB: 2048, DiskGB: 60, BandwidthMbit: 12, IP: "203.0.113.24", Prefix: 24, Gateway: "203.0.113.1",
		DNS: "8.8.8.8", SSHKeys: []string{"ssh-ed25519 AAAA test@pc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GET /api2/json/cluster/resources",
		"POST /api2/json/nodes/pve1/qemu/9000/clone",
		"PUT /api2/json/nodes/pve1/qemu/1001/config",
		"POST /api2/json/nodes/pve1/qemu/1001/firewall/ipset",
		"POST /api2/json/nodes/pve1/qemu/1001/firewall/ipset/ipfilter-net0",
		"GET /api2/json/nodes/pve1/qemu/1001/firewall/rules",
		"POST /api2/json/nodes/pve1/qemu/1001/firewall/rules",
		"PUT /api2/json/nodes/pve1/qemu/1001/firewall/options",
		"PUT /api2/json/nodes/pve1/qemu/1001/resize",
		"POST /api2/json/nodes/pve1/qemu/1001/status/start",
	}
	var got []string
	for _, c := range calls {
		if !strings.Contains(c, "/tasks/") {
			got = append(got, c)
		}
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls:\n%s", strings.Join(got, "\n"))
	}
	if configForm.Get("net0") != "virtio,bridge=vmbr0,firewall=1,rate=1.500" {
		t.Errorf("net0 = %q", configForm.Get("net0"))
	}
	if configForm.Get("ipconfig0") != "ip=203.0.113.24/24,gw=203.0.113.1" {
		t.Errorf("ipconfig0 = %q", configForm.Get("ipconfig0"))
	}
	if configForm.Get("sshkeys") != "ssh-ed25519%20AAAA%20test%40pc" {
		t.Errorf("sshkeys = %q", configForm.Get("sshkeys"))
	}
}

// SetFirewall köne düzgünleri pozmaly, täzelerini Proxmox-yň "başyna goşmak" tertibini
// hasaba alyp tersine ibermeli we syýasaty iň soňunda goýmaly.
func TestProxmoxSetFirewall(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	var posted []url.Values
	var options url.Values
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/firewall/rules"):
			posted = append(posted, form)
		case strings.HasSuffix(r.URL.Path, "/firewall/options"):
			options = form
		}
		mu.Unlock()
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/firewall/rules") {
			io.WriteString(w, `{"data":[{"pos":0},{"pos":1}]}`)
			return
		}
		io.WriteString(w, `{"data":null}`)
	}))
	defer srv.Close()

	p, err := NewProxmox(ProxmoxOptions{URL: srv.URL, TokenID: "a", TokenSecret: "b", InsecureTLS: true, Firewall: true})
	if err != nil {
		t.Fatal(err)
	}
	err = p.SetFirewall(context.Background(), "pve1", 1001, "203.0.113.24", Firewall{PolicyIn: "DROP", Rules: []FWRule{
		{Direction: "in", Action: "ACCEPT", Proto: "tcp", Dport: "22", Addr: "10.0.0.0/8", Comment: "SSH"},
		{Direction: "out", Action: "DROP", Addr: "198.51.100.7"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	deletes := 0
	for _, c := range calls {
		if c == "DELETE /api2/json/nodes/pve1/qemu/1001/firewall/rules/0" {
			deletes++
		}
	}
	if deletes != 2 {
		t.Errorf("köne düzgünleriň 2-si hem pozulmaly, pozuldy: %d", deletes)
	}
	if len(posted) != 3 {
		t.Fatalf("3 düzgün garaşylýar (SMTP + 2), goşuldy: %d", len(posted))
	}
	// Tersine tertip: iň soňky goşulan (SMTP) Proxmox-da iň ýokarda bolar
	if posted[0].Get("dest") != "198.51.100.7" || posted[0].Get("type") != "out" || posted[0].Get("source") != "" {
		t.Errorf("1-nji POST: %v", posted[0])
	}
	if posted[1].Get("source") != "10.0.0.0/8" || posted[1].Get("dport") != "22" || posted[1].Get("enable") != "1" {
		t.Errorf("2-nji POST: %v", posted[1])
	}
	if posted[2].Get("dport") != "25" || posted[2].Get("action") != "DROP" {
		t.Errorf("SMTP bloky iň soňunda goşulmaly (iň ýokarda durar): %v", posted[2])
	}
	if last := calls[len(calls)-1]; last != "PUT /api2/json/nodes/pve1/qemu/1001/firewall/options" {
		t.Errorf("syýasat iň soňunda goýulmaly, soňky haýyş: %s", last)
	}
	if options.Get("policy_in") != "DROP" || options.Get("ipfilter") != "1" {
		t.Errorf("options: %v", options)
	}
}

func TestProxmoxBackupFindsArchiveByNotes(t *testing.T) {
	var deletePath string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/vzdump"):
			io.WriteString(w, `{"data":"UPID:pve1:0002:vzdump:"}`)
		case strings.Contains(r.URL.Path, "/tasks/"):
			io.WriteString(w, `{"data":{"status":"stopped","exitstatus":"OK"}}`)
		case strings.HasSuffix(r.URL.Path, "/storage/local/content") && r.Method == http.MethodGet:
			if r.URL.Query().Get("vmid") != "1001" || r.URL.Query().Get("content") != "backup" {
				w.WriteHeader(400)
				return
			}
			io.WriteString(w, `{"data":[
				{"volid":"local:backup/vzdump-qemu-1001-new.vma.zst","size":999,"ctime":9999999999,"notes":"başga"},
				{"volid":"local:backup/vzdump-qemu-1001-mine.vma.zst","size":123,"ctime":1,"notes":"os.astu.tm bekap #7"}]}`)
		case r.Method == http.MethodDelete:
			deletePath = r.URL.EscapedPath()
			io.WriteString(w, `{"data":null}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	p, _ := NewProxmox(ProxmoxOptions{URL: srv.URL, TokenID: "a", TokenSecret: "b", InsecureTLS: true})
	info, err := p.Backup(context.Background(), "pve1", 1001, "local", "os.astu.tm bekap #7")
	if err != nil {
		t.Fatal(err)
	}
	if info.Volid != "local:backup/vzdump-qemu-1001-mine.vma.zst" || info.Size != 123 {
		t.Errorf("bellik boýunça tapylmaly: %+v", info)
	}
	if err := p.DeleteBackup(context.Background(), "pve1", info.Volid); err != nil {
		t.Fatal(err)
	}
	if deletePath != "/api2/json/nodes/pve1/storage/local/content/local:backup%2Fvzdump-qemu-1001-mine.vma.zst" {
		t.Errorf("DELETE ýoly: %s", deletePath)
	}
}
