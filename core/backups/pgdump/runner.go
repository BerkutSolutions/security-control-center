package pgdump

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

type DumpOptions struct {
	BinaryPath string
	DBURL      string
	OutputPath string
}

type Runner interface {
	Dump(ctx context.Context, opts DumpOptions) error
}

type CommandRunner struct{}

func NewRunner() *CommandRunner {
	return &CommandRunner{}
}

func (r *CommandRunner) Dump(ctx context.Context, opts DumpOptions) error {
	if strings.TrimSpace(opts.DBURL) == "" || strings.TrimSpace(opts.OutputPath) == "" {
		return fmt.Errorf("invalid pg_dump options")
	}
	bin := strings.TrimSpace(opts.BinaryPath)
	if bin == "" {
		bin = "pg_dump"
	}
	parsed, err := url.Parse(opts.DBURL)
	if err != nil {
		return err
	}
	host, port := parseHostPort(parsed)
	user := parsed.User.Username()
	pass, _ := parsed.User.Password()
	dbName := strings.TrimPrefix(parsed.Path, "/")
	if dbName == "" {
		return fmt.Errorf("database name is empty")
	}
	args := []string{
		"-Fc",
		"--no-owner",
		"--no-privileges",
		"-h", host,
		"-p", port,
		"-U", user,
		"-d", dbName,
		"-f", opts.OutputPath,
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	env := os.Environ()
	env = append(env, "PGPASSWORD="+pass)
	if sslMode := parsed.Query().Get("sslmode"); sslMode != "" {
		env = append(env, "PGSSLMODE="+sslMode)
	}
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 512 {
			msg = msg[:512]
		}
		if msg == "" {
			msg = "pg_dump execution failed"
		}
		return fmt.Errorf("%s: %w", msg, err)
	}
	return nil
}

func parseHostPort(u *url.URL) (string, string) {
	host := u.Host
	if strings.Contains(host, "@") {
		host = host[strings.LastIndex(host, "@")+1:]
	}
	h, p, err := net.SplitHostPort(host)
	if err == nil {
		if h == "" {
			h = "localhost"
		}
		if p == "" {
			p = "5432"
		}
		return h, p
	}
	if host == "" {
		host = "localhost"
	}
	return host, "5432"
}
