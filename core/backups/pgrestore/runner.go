package pgrestore

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Options struct {
	BinaryPath string
	DBURL      string
	InputPath  string
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
	cmd := exec.CommandContext(
		ctx,
		bin,
		"--clean",
		"--if-exists",
		"--exit-on-error",
		"--no-owner",
		"--no-privileges",
		"--dbname", options.DBURL,
		options.InputPath,
	)
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
