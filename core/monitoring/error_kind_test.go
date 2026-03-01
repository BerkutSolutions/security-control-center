package monitoring

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestClassifyAttemptError(t *testing.T) {
	if got := classifyAttemptError(context.DeadlineExceeded); got != ErrorKindTimeout {
		t.Fatalf("expected %q, got %q", ErrorKindTimeout, got)
	}

	if got := classifyAttemptError(ErrInvalidURL); got != ErrorKindInvalidURL {
		t.Fatalf("expected %q, got %q", ErrorKindInvalidURL, got)
	}

	if got := classifyAttemptError(&net.DNSError{Err: "no such host", Name: "example.invalid"}); got != ErrorKindDNS {
		t.Fatalf("expected %q, got %q", ErrorKindDNS, got)
	}

	if got := classifyAttemptError(errors.New("x509: certificate signed by unknown authority")); got != ErrorKindTLS {
		t.Fatalf("expected %q, got %q", ErrorKindTLS, got)
	}

	if got := classifyAttemptError(errors.New("connect: connection refused")); got != ErrorKindConnectionRefused {
		t.Fatalf("expected %q, got %q", ErrorKindConnectionRefused, got)
	}
}
