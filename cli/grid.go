package cli

import (
	"fmt"
	"strconv"
	"strings"
)

func parseFloatGrid(raw string) ([]float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty grid")
	}
	if strings.Contains(raw, ":") {
		parts := strings.Split(raw, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("expected range start:stop:step")
		}
		start, err := parseFloat(parts[0])
		if err != nil {
			return nil, err
		}
		stop, err := parseFloat(parts[1])
		if err != nil {
			return nil, err
		}
		step, err := parseFloat(parts[2])
		if err != nil {
			return nil, err
		}
		if step <= 0 {
			return nil, fmt.Errorf("step must be > 0")
		}
		var out []float64
		for v := start; v <= stop+1e-12; v += step {
			out = append(out, v)
			if len(out) > 10000 {
				return nil, fmt.Errorf("grid too large")
			}
		}
		return uniqFloat(out), nil
	}
	var out []float64
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := parseFloat(p)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty grid")
	}
	return uniqFloat(out), nil
}

func parseIntGrid(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty grid")
	}
	if strings.Contains(raw, ":") {
		parts := strings.Split(raw, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("expected range start:stop:step")
		}
		start, err := parseInt(parts[0])
		if err != nil {
			return nil, err
		}
		stop, err := parseInt(parts[1])
		if err != nil {
			return nil, err
		}
		step, err := parseInt(parts[2])
		if err != nil {
			return nil, err
		}
		if step <= 0 {
			return nil, fmt.Errorf("step must be > 0")
		}
		var out []int
		for v := start; v <= stop; v += step {
			out = append(out, v)
			if len(out) > 10000 {
				return nil, fmt.Errorf("grid too large")
			}
		}
		return uniqInt(out), nil
	}
	var out []int
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := parseInt(p)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty grid")
	}
	return uniqInt(out), nil
}

func parseFloat(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty number")
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("parse float %q: %w", raw, err)
	}
	return v, nil
}

func parseInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty number")
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parse int %q: %w", raw, err)
	}
	return v, nil
}

func uniqFloat(in []float64) []float64 {
	seen := map[string]struct{}{}
	var out []float64
	for _, v := range in {
		key := fmt.Sprintf("%.12f", v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
}

func uniqInt(in []int) []int {
	seen := map[int]struct{}{}
	var out []int
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

