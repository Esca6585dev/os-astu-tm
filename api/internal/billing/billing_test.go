package billing

import "testing"

func TestHourlyChargeNeverExceedsMonthly(t *testing.T) {
	for _, price := range []int64{14000, 17500, 29750, 42000, 59500} {
		var charged, n int64
		for h := 0; h < 744; h++ { // 31 günlik aý
			c := HourlyCharge(price, n, charged)
			if c < 0 {
				t.Fatalf("negative charge")
			}
			charged += c
			n++
		}
		if charged != price {
			t.Fatalf("price %d: charged %d, want exactly monthly cap", price, charged)
		}
	}
}

func TestHourlyChargeFirstHour(t *testing.T) {
	// VPS C-2020: 297.50 TMT → 29750 teňňe; 29750/720 = 41.3 → 41
	if got := HourlyCharge(29750, 0, 0); got != 41 {
		t.Fatalf("got %d want 41", got)
	}
	// 10 sagatdan soň jemi round(29750*10/720)=413
	var charged int64
	for n := int64(0); n < 10; n++ {
		charged += HourlyCharge(29750, n, charged)
	}
	if charged != 413 {
		t.Fatalf("10h charged %d want 413", charged)
	}
}

func TestFormatTMT(t *testing.T) {
	cases := map[int64]string{0: "0.00", 29750: "297.50", -40: "-0.40", 5: "0.05"}
	for in, want := range cases {
		if got := FormatTMT(in); got != want {
			t.Errorf("%d: got %s want %s", in, got, want)
		}
	}
}
