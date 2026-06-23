// Package ping measures ICMP latency per region and maps it to a status color.
// Note: pro-bing's PacketLoss is already a percentage (0-100).
package ping

import (
	"math"
	"net"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

type Color int

const (
	Green Color = iota
	Orange
	Red
	Grey
)

// ColorFor maps latency ms to a status color; -1 (failure) is grey.
func ColorFor(ms int) Color {
	switch {
	case ms <= 100 && ms != -1:
		return Green
	case ms > 100 && ms <= 200:
		return Orange
	case ms > 200:
		return Red
	default:
		return Grey
	}
}

type Result struct {
	RTTAvgMs float64
	LossPct  float64 // percentage 0-100, pro-bing native units
	OK       bool
	Fallback bool // true when measured via TCP connect, not ICMP
}

func failed() Result { return Result{RTTAvgMs: -1, LossPct: 100, OK: false} }

func (r Result) DisplayPing() int {
	if !r.OK {
		return -1
	}
	return int(r.RTTAvgMs)
}

func (r Result) DisplayLossPct() float64 {
	if !r.OK {
		return 100.0
	}
	frac := r.LossPct / 100.0
	rounded := math.Round(frac*100) / 100
	return rounded * 100
}

const (
	count   = 10
	perPing = 150 * time.Millisecond
	timeout = 3 * time.Second
)

// Ping measures latency via ICMP, falling back to TCP-connect to :443 when ICMP is unavailable.
func Ping(ip string) Result {
	if r := icmpPing(ip); r.OK {
		return r
	}
	return tcpFallback(ip)
}

func icmpPing(ip string) Result {
	pinger, err := probing.NewPinger(ip)
	if err != nil {
		return failed()
	}
	configurePinger(pinger)
	pinger.Count = count
	pinger.Interval = perPing
	pinger.Timeout = timeout
	if err := pinger.Run(); err != nil {
		return failed()
	}
	stats := pinger.Statistics()
	if stats.PacketsRecv == 0 {
		return failed()
	}
	return Result{
		RTTAvgMs: float64(stats.AvgRtt) / float64(time.Millisecond),
		LossPct:  stats.PacketLoss,
		OK:       true,
	}
}

func tcpFallback(ip string) Result {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "443"), timeout)
	if err != nil {
		return failed()
	}
	conn.Close()
	return Result{
		RTTAvgMs: float64(time.Since(start)) / float64(time.Millisecond),
		LossPct:  0,
		OK:       true,
		Fallback: true,
	}
}

// PingAll pings every IP concurrently, preserving input order.
func PingAll(ips []string) []Result {
	out := make([]Result, len(ips))
	var wg sync.WaitGroup
	for i, ip := range ips {
		wg.Add(1)
		go func(i int, ip string) {
			defer wg.Done()
			if ip == "" {
				out[i] = failed()
				return
			}
			out[i] = Ping(ip)
		}(i, ip)
	}
	wg.Wait()
	return out
}
