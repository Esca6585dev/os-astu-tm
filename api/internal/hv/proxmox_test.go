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
		"PUT /api2/json/nodes/pve1/qemu/1001/firewall/options",
		"POST /api2/json/nodes/pve1/qemu/1001/firewall/ipset",
		"POST /api2/json/nodes/pve1/qemu/1001/firewall/ipset/ipfilter-net0",
		"POST /api2/json/nodes/pve1/qemu/1001/firewall/rules",
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
