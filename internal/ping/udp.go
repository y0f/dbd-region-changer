package ping

import (
	"net"
	"time"
)

const beaconPort = "7770"

// beaconTimeout matches the game client's beacon wait.
const beaconTimeout = 2500 * time.Millisecond

// UDPBeacon sends a datagram to the GameLift ping beacon at ip:7770, which
// echoes back any non-empty payload, and reports the round-trip and whether it
// answered. Used to detect which regions are live (as the game client does).
func UDPBeacon(ip string) (rttMs float64, ok bool) {
	conn, err := net.DialTimeout("udp", net.JoinHostPort(ip, beaconPort), beaconTimeout)
	if err != nil {
		return -1, false
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(beaconTimeout))
	start := time.Now()
	if _, err := conn.Write([]byte("ping")); err != nil {
		return -1, false
	}
	buf := make([]byte, 512)
	if _, err := conn.Read(buf); err != nil {
		return -1, false
	}
	return float64(time.Since(start)) / float64(time.Millisecond), true
}
