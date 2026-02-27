package monitoring

import "testing"

func TestIsDNSErrorText(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"monitoring.error.timeout", false},
		{"monitoring.error.dnsNoAnswer", false},
		{"lookup example.invalid: no such host", true},
		{"dial tcp: lookup api.service.local on 127.0.0.11:53: no such host", true},
		{"getaddrinfo ENOTFOUND example.invalid", true},
		{"GetAddrInfo ENOTFOUND", true},
		{"temporary failure in name resolution", true},
		{"server misbehaving", true},
		{"NXDOMAIN", true},
		{"SERVFAIL", true},
		{"Name or service not known", true},
		{"connection refused", false},
		{"i/o timeout", false},
	}
	for _, tc := range cases {
		if got := isDNSErrorText(tc.in); got != tc.want {
			t.Fatalf("isDNSErrorText(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

