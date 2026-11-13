package httpapi

import (
	"testing"

	"github.com/Esca6585dev/os-astu-tm/api/internal/hv"
)

func TestCleanRule(t *testing.T) {
	ok := []struct {
		in   hv.FWRule
		want hv.FWRule
	}{
		{hv.FWRule{Direction: "IN", Action: "accept", Proto: "TCP", Dport: " 80, 443 ", Comment: " web\nserwer "},
			hv.FWRule{Direction: "in", Action: "ACCEPT", Proto: "tcp", Dport: "80,443", Comment: "web serwer"}},
		{hv.FWRule{Direction: "in", Action: "DROP", Proto: "udp", Dport: "8000:8100", Addr: "192.168.1.77/24"},
			hv.FWRule{Direction: "in", Action: "DROP", Proto: "udp", Dport: "8000:8100", Addr: "192.168.1.0/24"}},
		{hv.FWRule{Direction: "out", Action: "DROP", Proto: "icmp", Addr: "2001:db8::1"},
			hv.FWRule{Direction: "out", Action: "DROP", Proto: "icmp", Addr: "2001:db8::1"}},
	}
	for _, c := range ok {
		got, err := cleanRule(c.in)
		if err != nil || got != c.want {
			t.Errorf("cleanRule(%+v) = %+v, %v; garaşylýan %+v", c.in, got, err, c.want)
		}
	}
	bad := []hv.FWRule{
		{Direction: "up", Action: "ACCEPT"},
		{Direction: "in", Action: "REJECT"},
		{Direction: "in", Action: "ACCEPT", Proto: "gre"},
		{Direction: "in", Action: "ACCEPT", Proto: "icmp", Dport: "22"},
		{Direction: "in", Action: "ACCEPT", Dport: "22"},
		{Direction: "in", Action: "ACCEPT", Proto: "tcp", Dport: "0"},
		{Direction: "in", Action: "ACCEPT", Proto: "tcp", Dport: "70000"},
		{Direction: "in", Action: "ACCEPT", Proto: "tcp", Dport: "100:90"},
		{Direction: "in", Action: "ACCEPT", Proto: "tcp", Dport: "22;rm"},
		{Direction: "in", Action: "ACCEPT", Addr: "10.0.0.300"},
		{Direction: "in", Action: "ACCEPT", Addr: "+dc/ipset"},
	}
	for _, r := range bad {
		if _, err := cleanRule(r); err == nil {
			t.Errorf("cleanRule(%+v) ýalňyşlyk bermeli", r)
		}
	}
}

func TestTruncateKeepsUTF8(t *testing.T) {
	// "ň" 2 baýt: 3 baýta kesilende harp bölünmeli däl
	if got := truncate("aňb", 2); got != "a" {
		t.Errorf("truncate = %q", got)
	}
	if got := cleanText("  Aşgabat\t\x00 şäheri  ", 100); got != "Aşgabat şäheri" {
		t.Errorf("cleanText = %q", got)
	}
}
