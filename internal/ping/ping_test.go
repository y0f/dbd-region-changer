package ping

import "testing"

func TestSentinelOnFailure(t *testing.T) {
	r := Result{} // OK == false
	if r.DisplayPing() != -1 {
		t.Errorf("DisplayPing=%d want -1", r.DisplayPing())
	}
	if r.DisplayLossPct() != 100.0 {
		t.Errorf("DisplayLossPct=%v want 100.0", r.DisplayLossPct())
	}
}

func TestLossPercentSuccessPath(t *testing.T) {
	r := Result{RTTAvgMs: 42, LossPct: 50.0, OK: true}
	if r.DisplayPing() != 42 {
		t.Errorf("DisplayPing=%d want 42", r.DisplayPing())
	}
	if r.DisplayLossPct() != 50.0 {
		t.Errorf("DisplayLossPct=%v want 50.0", r.DisplayLossPct())
	}
	r2 := Result{RTTAvgMs: 42, LossPct: 33.6, OK: true}
	if r2.DisplayLossPct() != 34.0 {
		t.Errorf("DisplayLossPct=%v want 34.0", r2.DisplayLossPct())
	}
	r3 := Result{RTTAvgMs: 10, LossPct: 0, OK: true}
	if r3.DisplayLossPct() != 0.0 {
		t.Errorf("DisplayLossPct=%v want 0.0", r3.DisplayLossPct())
	}
}
