package main

import (
	"context"
	"net"
	"sort"
	"time"
)

// snapshot holds one window of monitoring results for a zone.
type snapshot struct {
	checks      int
	success     int
	latencies   []float64 // ms, successful lookups only
	targetsUp   int
	targetsDown int
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p / 100 * float64(len(sorted)-1))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// resolverFor returns a *net.Resolver that queries a specific DNS server.
func resolverFor(server string) *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, network, net.JoinHostPort(server, "53"))
		},
	}
}

// monitorZone resolves the apex across resolvers (measuring latency) and dials the
// resolved targets on :443 to gauge reachability.
func monitorZone(ctx context.Context, zone string, resolvers []string, dialTimeout time.Duration) snapshot {
	var s snapshot
	var ips []string

	for _, server := range resolvers {
		r := resolverFor(server)
		start := time.Now()
		lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		addrs, err := r.LookupHost(lookupCtx, zone)
		cancel()
		s.checks++
		if err != nil || len(addrs) == 0 {
			continue
		}
		s.success++
		s.latencies = append(s.latencies, float64(time.Since(start).Microseconds())/1000.0)
		if len(ips) == 0 {
			ips = addrs
		}
	}

	// Target reachability: dial each resolved IP on 443.
	for _, ip := range ips {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "443"), dialTimeout)
		if err != nil {
			s.targetsDown++
			continue
		}
		_ = conn.Close()
		s.targetsUp++
	}
	return s
}

func (s snapshot) toPayload(zone, region, agentID string, start, end time.Time) metricPayload {
	successRate := 0.0
	if s.checks > 0 {
		successRate = float64(s.success) / float64(s.checks) * 100
	}
	sorted := append([]float64(nil), s.latencies...)
	sort.Float64s(sorted)
	return metricPayload{
		ZoneName:    zone,
		Region:      region,
		AgentID:     agentID,
		WindowStart: start.UTC().Format(time.RFC3339),
		WindowEnd:   end.UTC().Format(time.RFC3339),
		Checks:      s.checks,
		SuccessRate: successRate,
		LatencyP50:  percentile(sorted, 50),
		LatencyP95:  percentile(sorted, 95),
		TargetsUp:   s.targetsUp,
		TargetsDown: s.targetsDown,
	}
}
