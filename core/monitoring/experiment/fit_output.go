package experiment

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func WriteFitResult(outPath string, res FitResult) error {
	if strings.TrimSpace(outPath) == "" {
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	ext := strings.ToLower(filepath.Ext(outPath))
	if ext == ".csv" {
		f, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer f.Close()
		return writeFitCSV(f, res)
	}
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, b, 0644)
}

func writeFitCSV(w io.Writer, res FitResult) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write([]string{
		"rank",
		"policy",
		"open_threshold",
		"close_threshold",
		"confirmations",
		"hmm3_diag_boost",
		"loss",
		"false_opens",
		"misses",
		"delay_sec_sum",
		"actions",
		"generated_at",
	})
	for i, c := range res.Top {
		row := []string{
			fmt.Sprintf("%d", i+1),
			string(c.Policy),
			fmt.Sprintf("%.4f", c.Config.OpenThreshold),
			fmt.Sprintf("%.4f", c.Config.CloseThreshold),
			fmt.Sprintf("%d", c.Config.Confirmations),
			fmt.Sprintf("%.4f", c.Config.HMM3DiagBoost),
			fmt.Sprintf("%.6f", c.Loss.Value),
			fmt.Sprintf("%d", c.Loss.FalseOpens),
			fmt.Sprintf("%d", c.Loss.Misses),
			fmt.Sprintf("%.3f", c.Loss.DelaySecSum),
			fmt.Sprintf("%d", c.Loss.Actions),
			res.GeneratedAt.Format(time.RFC3339),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	return cw.Error()
}

