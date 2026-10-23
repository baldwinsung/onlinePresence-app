package main

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"errors"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	whoisparser "github.com/likexian/whois-parser"
)

func fakeLookups() lookups {
	created := time.Date(1997, 9, 15, 4, 0, 0, 0, time.UTC)
	expires := time.Date(2028, 9, 14, 4, 0, 0, 0, time.UTC)
	return lookups{
		whois: func(_ context.Context, d string) (whoisparser.WhoisInfo, error) {
			if d != "google.com" {
				return whoisparser.WhoisInfo{}, whoisparser.ErrNotFoundDomain
			}
			return whoisparser.WhoisInfo{
				Registrar: &whoisparser.Contact{Name: "MarkMonitor, Inc."},
				Domain:    &whoisparser.Domain{CreatedDateInTime: &created, ExpirationDateInTime: &expires},
			}, nil
		},
		resolveA: func(_ context.Context, d string) ([]net.IP, error) {
			if d == "noa.example" {
				return nil, nil
			}
			return []net.IP{net.ParseIP("142.250.80.46")}, nil
		},
		rdapName: func(context.Context, net.IP) (string, error) { return "Google LLC", nil },
		cert: func(_ context.Context, d string) (*x509.Certificate, error) {
			if d == "down.example" {
				return nil, errors.New("dial tcp: connection refused")
			}
			c := &x509.Certificate{
				NotBefore: time.Date(2026, 9, 28, 18, 38, 33, 0, time.UTC),
				NotAfter:  time.Date(2026, 12, 21, 18, 38, 32, 0, time.UTC),
				Subject: pkix.Name{Names: []pkix.AttributeTypeAndValue{
					{Type: asn1.ObjectIdentifier{2, 5, 4, 6}, Value: "US"},
					{Type: oidCommonName, Value: "*.meta.com"},
				}},
				Issuer: pkix.Name{Names: []pkix.AttributeTypeAndValue{
					{Type: asn1.ObjectIdentifier{2, 5, 4, 6}, Value: "US"},
					{Type: oidOrganization, Value: "Google Trust Services"},
					{Type: oidCommonName, Value: "WR2"},
				}},
				DNSNames:    []string{"meta.com", "*.meta.com"},
				IPAddresses: []net.IP{net.ParseIP("10.0.0.1")},
			}
			if d == "nosan.example" {
				c.DNSNames, c.IPAddresses = nil, nil
			}
			return c, nil
		},
	}
}

func TestRoutes(t *testing.T) {
	h := newMux(fakeLookups())
	for _, tc := range []struct {
		method, path string
		code         int
		want         string
	}{
		{"GET", "/", 200, `"main page, enter the target path"`},
		{"GET", "/health", 200, `{"status":"ok"}`},
		{"PUT", "/domainwhois_registrar/google.com", 200, `"MarkMonitor, Inc."`},
		{"PUT", "/domainwhois_created/google.com", 200, `"Mon, 15 Sep 1997 04:00:00 GMT"`},
		{"PUT", "/domainwhois_expires/google.com", 200, `"Thu, 14 Sep 2028 04:00:00 GMT"`},
		{"PUT", "/domainwhois_registrar/nope.example", 502, `{"error":"whoisparser: domain is not found"}`},
		{"PUT", "/hostingprovider/google.com", 200, `"Google LLC"`},
		{"PUT", "/hostingprovider/noa.example", 404, `{"error":"No A record found"}`},
		{"PUT", "/sslcertificate_subject/meta.com", 200, `"CN=*.meta.com"`},
		{"PUT", "/sslcertificate_issuer/google.com", 200, `"O=Google Trust Services"`},
		{"PUT", "/sslcertificate_notbefore/google.com", 200, `"Mon, 28 Sep 2026 18:38:33 GMT"`},
		{"PUT", "/sslcertificate_notafter/google.com", 200, `"Mon, 21 Dec 2026 18:38:32 GMT"`},
		{"PUT", "/sslcertificate_san/meta.com", 200, `["DNS:*.meta.com","DNS:meta.com","IP Address:10.0.0.1"]`},
		{"PUT", "/sslcertificate_san/nosan.example", 404, `{"error":"No SAN extension found"}`},
		{"PUT", "/sslcertificate_subject/down.example", 502, `{"error":"dial tcp: connection refused"}`},
		{"GET", "/hostingprovider/google.com", 405, ""},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != tc.code {
			t.Errorf("%s %s: status %d, want %d", tc.method, tc.path, rec.Code, tc.code)
		}
		if tc.want == "" {
			continue
		}
		if got := strings.TrimSpace(rec.Body.String()); got != tc.want {
			t.Errorf("%s %s: body %s, want %s", tc.method, tc.path, got, tc.want)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("%s %s: Content-Type %q", tc.method, tc.path, ct)
		}
	}
}

func TestRDAPEntityName(t *testing.T) {
	var r rdapResponse
	raw := `{"entities":[
		{"handle":"X","roles":["abuse"]},
		{"handle":"GOGL","vcardArray":["vcard",[["version",{},"text","4.0"],["fn",{},"text","Google LLC"]]]}
	]}`
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatal(err)
	}
	if got := r.Entities[0].fn(); got != "" {
		t.Errorf("entity without vCard: %q", got)
	}
	if got := r.Entities[1].fn(); got != "Google LLC" {
		t.Errorf("fn = %q", got)
	}
}
