package cli

import (
	"context"
	"flag"
	"path/filepath"
	"strings"
	"time"

	"berkut-scc/config"
	monexp "berkut-scc/core/monitoring/experiment"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func runExperiment(args []string) {
	cmd := flag.NewFlagSet("experiment", flag.ExitOnError)
	monitorID := cmd.Int64("monitor-id", 0, "monitor id")
	sinceRaw := cmd.String("since", "", "since date (YYYY-MM-DD or RFC3339)")
	modeRaw := cmd.String("mode", "replay", "replay|simulate")
	outRaw := cmd.String("out", "", "output path (.json or .csv); empty prints JSON to stdout")
	scenarioRaw := cmd.String("scenario", "mixed", "simulate scenario: mixed|outage|flap|degrade")
	seed := cmd.Int64("seed", 1, "simulate seed")
	durationRaw := cmd.String("duration", "60m", "simulate duration (e.g. 30m, 2h)")
	stepRaw := cmd.String("step", "30s", "simulate step (e.g. 10s, 30s, 1m)")
	shortOutageRaw := cmd.String("short-outage", "120s", "replay false-open heuristic: outages shorter than this are likely false opens")
	_ = cmd.Parse(args)

	cfg, _ := config.Load()
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		logger.Fatalf("db: %v", err)
	}
	defer db.Close()

	if *monitorID <= 0 {
		logger.Fatalf("experiment: --monitor-id is required")
	}
	since, err := parseSince(*sinceRaw)
	if err != nil {
		logger.Fatalf("experiment: invalid --since: %v", err)
	}
	shortOutage, err := time.ParseDuration(*shortOutageRaw)
	if err != nil {
		logger.Fatalf("experiment: invalid --short-outage: %v", err)
	}

	ms := store.NewMonitoringStore(db)
	mon, err := ms.GetMonitor(context.Background(), *monitorID)
	if err != nil || mon == nil {
		logger.Fatalf("experiment: monitor not found: %v", err)
	}
	settings, err := ms.GetSettings(context.Background())
	if err != nil || settings == nil {
		logger.Fatalf("experiment: monitoring settings: %v", err)
	}

	mode := strings.ToLower(strings.TrimSpace(*modeRaw))
	runCfg := monexp.RunConfig{
		Mode:              monexp.Mode(mode),
		Monitor:           *mon,
		Settings:          *settings,
		Since:             since,
		ShortOutageWindow: shortOutage,
	}

	switch runCfg.Mode {
	case monexp.ModeReplay:
		metrics, err := ms.ListMetrics(context.Background(), *monitorID, since)
		if err != nil {
			logger.Fatalf("experiment: list metrics: %v", err)
		}
		runCfg.Replay = make([]monexp.Observation, 0, len(metrics))
		for _, m := range metrics {
			var errText string
			if m.Error != nil {
				errText = *m.Error
			}
			runCfg.Replay = append(runCfg.Replay, monexp.Observation{
				TS:         m.TS.UTC(),
				LatencyMs:  m.LatencyMs,
				OK:         m.OK,
				StatusCode: m.StatusCode,
				Error:      errText,
			})
		}
	case monexp.ModeSimulate:
		dur, err := time.ParseDuration(*durationRaw)
		if err != nil {
			logger.Fatalf("experiment: invalid --duration: %v", err)
		}
		step, err := time.ParseDuration(*stepRaw)
		if err != nil {
			logger.Fatalf("experiment: invalid --step: %v", err)
		}
		scMode := monexp.ScenarioMode(strings.ToLower(strings.TrimSpace(*scenarioRaw)))
		sc := monexp.ScenarioConfig{
			Mode:     scMode,
			Seed:     *seed,
			StartAt:  since,
			Duration: dur,
			Step:     step,
			Latency:  120,
		}
		runCfg.Simulate = monexp.GenerateScenario(sc)
		runCfg.Scenario = &sc
	default:
		logger.Fatalf("experiment: unknown --mode: %s", mode)
	}

	res, err := monexp.Run(context.Background(), runCfg)
	if err != nil {
		logger.Fatalf("experiment: %v", err)
	}
	out := strings.TrimSpace(*outRaw)
	if out != "" && !filepath.IsAbs(out) {
		out = filepath.Clean(out)
	}
	if err := monexp.WriteResult(out, res); err != nil {
		logger.Fatalf("experiment: write result: %v", err)
	}
}

