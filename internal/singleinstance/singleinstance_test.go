package singleinstance

import "testing"

func TestSecondAcquireReportsRunning(t *testing.T) {
	const name = "dbd_region_changer_test_lock"
	a, err := Acquire(name)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Release()
	if a.IsRunning() {
		t.Fatal("first acquire should not report another instance running")
	}

	b, err := Acquire(name)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Release()
	if !b.IsRunning() {
		t.Fatal("second acquire should report an instance already running")
	}
}
