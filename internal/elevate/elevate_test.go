package elevate

import (
	"encoding/json"
	"testing"

	"github.com/y0f/dbd-region-changer/internal/hostsfile"
)

func TestPayloadRoundTrip(t *testing.T) {
	p := Payload{
		Op:              OpWrite,
		RemoveHostnames: []string{"gamelift.us-east-1.amazonaws.com"},
		Entries: []hostsfile.Entry{
			{IP: "1.2.3.4", Hostname: "gamelift.us-east-1.amazonaws.com"},
		},
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got Payload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Op != OpWrite || len(got.Entries) != 1 || got.Entries[0].IP != "1.2.3.4" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if len(got.RemoveHostnames) != 1 {
		t.Fatalf("remove hostnames lost: %+v", got)
	}
}

func TestHandleHelperSubcommandIgnoresNonHelper(t *testing.T) {
	handled, err := HandleHelperSubcommand([]string{"--debug"})
	if handled || err != nil {
		t.Fatalf("non-helper arg should be ignored: handled=%v err=%v", handled, err)
	}
	handled, err = HandleHelperSubcommand(nil)
	if handled || err != nil {
		t.Fatalf("no args should be ignored: handled=%v err=%v", handled, err)
	}
}

func TestIsElevatedCallable(t *testing.T) {
	_ = IsElevated()
}
