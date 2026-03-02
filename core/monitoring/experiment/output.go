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

func WriteResult(outPath string, res Result) error {
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
		return writeCSV(f, res)
	}
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, b, 0644)
}

func writeCSV(w io.Writer, res Result) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write([]string{
		"policy",
		"outage_index",
		"started_at",
		"ended_at",
		"duration_sec",
		"opened_at",
		"closed_at",
		"time_to_open_sec",
		"false_open",
	})
	for _, p := range res.Policies {
		for _, o := range p.Outages {
			row := []string{
				p.Name,
				fmt.Sprintf("%d", o.Index),
				o.StartedAt.Format(time.RFC3339),
				o.EndedAt.Format(time.RFC3339),
				fmt.Sprintf("%d", o.DurationSec),
				"",
				"",
				"",
				fmt.Sprintf("%v", o.FalseOpen),
			}
			if o.OpenedAt != nil {
				row[5] = o.OpenedAt.Format(time.RFC3339)
			}
			if o.ClosedAt != nil {
				row[6] = o.ClosedAt.Format(time.RFC3339)
			}
			if o.TimeToOpenSec != nil {
				row[7] = fmt.Sprintf("%d", *o.TimeToOpenSec)
			}
			if err := cw.Write(row); err != nil {
				return err
			}
		}
	}
	return cw.Error()
}
