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

func runExperimentFit(args []string) {
	cmd := flag.NewFlagSet("experiment-fit", flag.ExitOnError)
	monitorID := cmd.Int64("monitor-id", 0, "monitor id")
	sinceRaw := cmd.String("since", "", "since date (YYYY-MM-DD or RFC3339)")
	modeRaw := cmd.String("mode", "replay", "replay|simulate")
	outRaw := cmd.String("out", "", "output path (.json or .csv); empty prints JSON to stdout")
	scenarioRaw := cmd.String("scenario", "mixed", "simulate scenario: mixed|outage|flap|degrade")
	seed := cmd.Int64("seed", 1, "simulate seed")
	durationRaw := cmd.String("duration", "60m", "simulate duration (e.g. 30m, 2h)")
	stepRaw := cmd.String("step", "30s", "simulate step (e.g. 10s, 30s, 1m)")
	shortOutageRaw := cmd.String("short-outage", "120s", "replay heuristics: outages shorter than this are 'short' (likely false opens)")
	topN := cmd.Int("top", 10, "top N candidates to output")
	openGridRaw := cmd.String("open", "0.60:0.95:0.05", "grid for open threshold: list '0.7,0.8' or range 'start:stop:step'")
	closeGridRaw := cmd.String("close", "0.05:0.50:0.05", "grid for close threshold: list '0.1,0.2' or range 'start:stop:step'")
	confGridRaw := cmd.String("confirm", "1,2,3,4,5", "grid for confirmations: list '1,2,3' or range 'start:stop:step'")
	hmmBoostRaw := cmd.String("hmm-boost", "0,0.5,1,2", "grid for HMM3 diag boost (state stickiness); only for hmm3")
	wFalse := cmd.Float64("w-false", 10, "loss weight: false opens")
	wMiss := cmd.Float64("w-miss", 50, "loss weight: misses")
	wDelay := cmd.Float64("w-delay", 0.05, "loss weight: time-to-open (per second, summed)")
	wNoise := cmd.Float64("w-noise", 1, "loss weight: actions (opens+closes)")
	_ = cmd.Parse(args)

	cfg, _ := config.Load()
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		logger.Fatalf("db: %v", err)
	}
	defer db.Close()

	if *monitorID <= 0 {
		logger.Fatalf("experiment-fit: --monitor-id is required")
	}
	since, err := parseSince(*sinceRaw)
	if err != nil {
		logger.Fatalf("experiment-fit: invalid --since: %v", err)
	}
	shortOutage, err := time.ParseDuration(*shortOutageRaw)
	if err != nil {
		logger.Fatalf("experiment-fit: invalid --short-outage: %v", err)
	}
	openGrid, err := parseFloatGrid(*openGridRaw)
	if err != nil {
		logger.Fatalf("experiment-fit: invalid --open: %v", err)
	}
	closeGrid, err := parseFloatGrid(*closeGridRaw)
	if err != nil {
		logger.Fatalf("experiment-fit: invalid --close: %v", err)
	}
	confGrid, err := parseIntGrid(*confGridRaw)
	if err != nil {
		logger.Fatalf("experiment-fit: invalid --confirm: %v", err)
	}
	boostGrid, err := parseFloatGrid(*hmmBoostRaw)
	if err != nil {
		logger.Fatalf("experiment-fit: invalid --hmm-boost: %v", err)
	}

	ms := store.NewMonitoringStore(db)
	mon, err := ms.GetMonitor(context.Background(), *monitorID)
	if err != nil || mon == nil {
		logger.Fatalf("experiment-fit: monitor not found: %v", err)
	}
	settings, err := ms.GetSettings(context.Background())
	if err != nil || settings == nil {
		logger.Fatalf("experiment-fit: monitoring settings: %v", err)
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
			logger.Fatalf("experiment-fit: list metrics: %v", err)
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
			logger.Fatalf("experiment-fit: invalid --duration: %v", err)
		}
		step, err := time.ParseDuration(*stepRaw)
		if err != nil {
			logger.Fatalf("experiment-fit: invalid --step: %v", err)
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
		logger.Fatalf("experiment-fit: unknown --mode: %s", mode)
	}

	fc := monexp.FitConfig{
		Weights: monexp.LossWeights{
			FalseOpen: *wFalse,
			Miss:      *wMiss,
			DelaySec:  *wDelay,
			Noise:     *wNoise,
		},
		Grid: monexp.FitGrid{
			OpenThresholds:  openGrid,
			CloseThresholds: closeGrid,
			Confirmations:   confGrid,
			HMM3DiagBoosts:  boostGrid,
		},
		TopN: *topN,
	}
	res, err := monexp.Fit(context.Background(), runCfg, fc)
	if err != nil {
		logger.Fatalf("experiment-fit: %v", err)
	}
	out := strings.TrimSpace(*outRaw)
	if out != "" && !filepath.IsAbs(out) {
		out = filepath.Clean(out)
	}
	if err := monexp.WriteFitResult(out, res); err != nil {
		logger.Fatalf("experiment-fit: write result: %v", err)
	}
}

