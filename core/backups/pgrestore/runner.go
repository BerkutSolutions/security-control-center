package pgrestore

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Options struct {
	BinaryPath      string
	DBURL           string
	InputPath       string
	Clean           bool
	DataOnly        bool
	Tables          []string
	DisableTriggers bool
}

type Runner interface {
	Restore(ctx context.Context, options Options) error
}

type runner struct{}

func NewRunner() Runner {
	return &runner{}
}

func (r *runner) Restore(ctx context.Context, options Options) error {
	bin := strings.TrimSpace(options.BinaryPath)
	if bin == "" {
		bin = "pg_restore"
	}
	args := []string{
		"--exit-on-error",
		"--no-owner",
		"--no-privileges",
	}
	if options.Clean {
		args = append(args, "--clean", "--if-exists")
	}
	if options.DataOnly {
		args = append(args, "--data-only")
	}
	if options.DisableTriggers {
		args = append(args, "--disable-triggers")
	}
	for _, raw := range options.Tables {
		table := strings.TrimSpace(raw)
		if table == "" {
			continue
		}
		if strings.Contains(table, ".") {
			args = append(args, "--table", table)
			continue
		}
		args = append(args, "--table", "public."+table)
	}
	args = append(args, "--dbname", options.DBURL, options.InputPath)
	cmd := exec.CommandContext(ctx, bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, sanitizeStderr(msg))
	}
	return nil
}

func sanitizeStderr(in string) string {
	if len(in) > 512 {
		return in[:512]
	}
	return in
}
