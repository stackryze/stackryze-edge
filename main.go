package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type config struct {
	apiURL      string
	token       string
	region      string
	agentID     string
	interval    time.Duration
	zones       []string
	resolvers   []string
	dialTimeout time.Duration
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func loadConfig() config {
	host, _ := os.Hostname()
	region := env("STACKRYZE_EDGE_REGION", "default")
	interval, _ := strconv.Atoi(env("STACKRYZE_EDGE_INTERVAL", "60"))
	if interval < 15 {
		interval = 15
	}
	resolvers := splitList(env("STACKRYZE_EDGE_RESOLVERS", "8.8.8.8,1.1.1.1,9.9.9.9"))
	return config{
		apiURL:      env("STACKRYZE_API_URL", "https://api.stackryze.com/api"),
		token:       os.Getenv("STACKRYZE_API_TOKEN"),
		region:      region,
		agentID:     env("STACKRYZE_EDGE_AGENT_ID", region+"-"+host),
		interval:    time.Duration(interval) * time.Second,
		zones:       splitList(os.Getenv("STACKRYZE_EDGE_ZONES")),
		resolvers:   resolvers,
		dialTimeout: 4 * time.Second,
	}
}

func main() {
	cfg := loadConfig()
	if cfg.token == "" {
		log.Fatal("STACKRYZE_API_TOKEN is required")
	}
	client := newClient(cfg.apiURL, cfg.token)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("Stackryze Edge agent starting — region=%s agent=%s interval=%s", cfg.region, cfg.agentID, cfg.interval)

	runCycle(ctx, client, cfg) // run once immediately
	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("shutting down")
			return
		case <-ticker.C:
			runCycle(ctx, client, cfg)
		}
	}
}

func runCycle(ctx context.Context, client *stackryzeClient, cfg config) {
	zones := cfg.zones
	if len(zones) == 0 {
		discovered, err := client.discoverZones(ctx)
		if err != nil {
			log.Printf("zone discovery failed: %v", err)
			return
		}
		zones = discovered
	}
	if len(zones) == 0 {
		log.Println("no zones to monitor")
		return
	}

	start := time.Now()
	for _, zone := range zones {
		snap := monitorZone(ctx, zone, cfg.resolvers, cfg.dialTimeout)
		payload := snap.toPayload(zone, cfg.region, cfg.agentID, start, time.Now())
		if err := client.pushMetrics(ctx, payload); err != nil {
			log.Printf("push failed for %s: %v", zone, err)
			continue
		}
		log.Printf("%s: checks=%d success=%.0f%% p50=%.1fms p95=%.1fms up=%d down=%d",
			zone, payload.Checks, payload.SuccessRate, payload.LatencyP50, payload.LatencyP95, payload.TargetsUp, payload.TargetsDown)
	}
}
