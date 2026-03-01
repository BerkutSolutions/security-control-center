package netguard

import (
	"context"
	"testing"
	"time"
)

func TestValidateHost_BlocksMetadataEvenWhenPrivateAllowed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := ValidateHost(ctx, "169.254.169.254", Policy{AllowPrivate: true, AllowLoopback: true}); err == nil {
		t.Fatalf("expected metadata to be blocked")
	}
}

func TestValidateHost_AllowsRFC1918WhenPrivateAllowed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := ValidateHost(ctx, "10.0.0.1", Policy{AllowPrivate: true}); err != nil {
		t.Fatalf("expected rfc1918 ip to be allowed, got %v", err)
	}
}

func TestValidateHost_BlocksRFC1918WhenPrivateNotAllowed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := ValidateHost(ctx, "10.0.0.1", Policy{AllowPrivate: false}); err == nil {
		t.Fatalf("expected rfc1918 ip to be blocked")
	}
}

func TestValidateHost_AllowsLoopbackOnlyWhenEnabled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := ValidateHost(ctx, "127.0.0.1", Policy{AllowPrivate: true, AllowLoopback: false}); err == nil {
		t.Fatalf("expected loopback to be blocked when AllowLoopback=false")
	}
	if err := ValidateHost(ctx, "127.0.0.1", Policy{AllowPrivate: true, AllowLoopback: true}); err != nil {
		t.Fatalf("expected loopback to be allowed when AllowLoopback=true, got %v", err)
	}
}
