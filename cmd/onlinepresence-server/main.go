// Command onlinepresence-server is the onlinePresence REST API: WHOIS
// registration, hosting provider and TLS certificate details for a domain,
// one PUT endpoint each. It replaced the Flask app (app.py) on 2026-10-23
// so the whole VPS fleet runs small Go binaries; routes, methods and JSON
// shapes are unchanged (see README for the few deliberate differences).
//
// No database and no state: every request is a live outbound lookup.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] != "serve" {
		log.Fatalf("unknown command %q (only \"serve\")", os.Args[1])
	}

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           newMux(liveLookups()),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("onlinepresence-server listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}

// lookups are the outbound calls, swappable in tests.
type lookups struct {
	whois    func(ctx context.Context, domain string) (whoisparser.WhoisInfo, error)
	resolveA func(ctx context.Context, domain string) ([]net.IP, error)
	rdapName func(ctx context.Context, ip net.IP) (string, error)
	cert     func(ctx context.Context, domain string) (*x509.Certificate, error)
}

const lookupTimeout = 15 * time.Second

func liveLookups() lookups {
	whoisClient := whois.NewClient().SetTimeout(lookupTimeout)
	return lookups{
		whois: func(ctx context.Context, domain string) (whoisparser.WhoisInfo, error) {
			raw, err := whoisClient.Whois(domain)
			if err != nil {
				return whoisparser.WhoisInfo{}, err
			}
			return whoisparser.Parse(raw)
		},
		resolveA: func(ctx context.Context, domain string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip4", domain)
		},
		rdapName: rdapFirstEntityName,
		cert:     serverCert,
	}
}

func newMux(l lookups) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, "main page, enter the target path")
	})

	whoisRoute := func(name string, pick func(whoisparser.WhoisInfo) any) {
		mux.HandleFunc("PUT /"+name+"/{domain}", func(w http.ResponseWriter, r *http.Request) {
			info, err := l.whois(r.Context(), r.PathValue("domain"))
			if err != nil {
				writeError(w, http.StatusBadGateway, err)
				return
			}
			writeJSON(w, http.StatusOK, pick(info))
		})
	}
	whoisRoute("domainwhois_registrar", func(i whoisparser.WhoisInfo) any {
		if i.Registrar == nil || i.Registrar.Name == "" {
			return nil
		}
		return i.Registrar.Name
	})
	whoisRoute("domainwhois_created", func(i whoisparser.WhoisInfo) any {
		if i.Domain == nil {
			return nil
		}
		return httpDate(i.Domain.CreatedDateInTime)
	})
	whoisRoute("domainwhois_expires", func(i whoisparser.WhoisInfo) any {
		if i.Domain == nil {
			return nil
		}
		return httpDate(i.Domain.ExpirationDateInTime)
	})

	mux.HandleFunc("PUT /hostingprovider/{domain}", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), lookupTimeout)
		defer cancel()
		ips, err := l.resolveA(ctx, r.PathValue("domain"))
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		if len(ips) == 0 {
			writeError(w, http.StatusNotFound, errors.New("No A record found"))
			return
		}
		name, err := l.rdapName(ctx, ips[0])
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		if name == "" {
			writeError(w, http.StatusNotFound, errors.New("Hosting provider not found"))
			return
		}
		writeJSON(w, http.StatusOK, name)
	})

	certRoute := func(name string, pick func(*x509.Certificate) (any, error)) {
		mux.HandleFunc("PUT /"+name+"/{domain}", func(w http.ResponseWriter, r *http.Request) {
			c, err := l.cert(r.Context(), r.PathValue("domain"))
			if err != nil {
				writeError(w, http.StatusBadGateway, err)
				return
			}
			v, err := pick(c)
			if err != nil {
				writeError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, v)
		})
	}
	certRoute("sslcertificate_subject", func(c *x509.Certificate) (any, error) {
		return nameAttr(c.Subject, oidCommonName), nil
	})
	certRoute("sslcertificate_issuer", func(c *x509.Certificate) (any, error) {
		return nameAttr(c.Issuer, oidOrganization), nil
	})
	certRoute("sslcertificate_notbefore", func(c *x509.Certificate) (any, error) {
		return httpDate(&c.NotBefore), nil
	})
	certRoute("sslcertificate_notafter", func(c *x509.Certificate) (any, error) {
		return httpDate(&c.NotAfter), nil
	})
	certRoute("sslcertificate_san", func(c *x509.Certificate) (any, error) {
		sans := subjectAltNames(c)
		if len(sans) == 0 {
			return nil, errors.New("No SAN extension found")
		}
		return sans, nil
	})
	return mux
}

// httpDate formats like Flask's jsonify did for a datetime:
// "Mon, 15 Sep 1997 04:00:00 GMT".
func httpDate(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC().Format(http.TimeFormat)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// serverCert returns the leaf certificate domain:443 presents for its own
// name. Not verified, like Python's ssl.get_server_certificate: the point is
// to report what's there, expired or not.
func serverCert(ctx context.Context, domain string) (*x509.Certificate, error) {
	d := tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 5 * time.Second},
		Config:    &tls.Config{ServerName: domain, InsecureSkipVerify: true},
	}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(domain, "443"))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	certs := conn.(*tls.Conn).ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, errors.New("no certificate presented")
	}
	return certs[0], nil
}

var (
	oidCommonName   = asn1.ObjectIdentifier{2, 5, 4, 3}
	oidOrganization = asn1.ObjectIdentifier{2, 5, 4, 10}
)

// shortNames are OpenSSL's short names for the usual DN attributes.
var shortNames = map[string]string{
	"2.5.4.3":              "CN",
	"2.5.4.6":              "C",
	"2.5.4.7":              "L",
	"2.5.4.8":              "ST",
	"2.5.4.10":             "O",
	"2.5.4.11":             "OU",
	"2.5.4.5":              "serialNumber",
	"1.2.840.113549.1.9.1": "emailAddress",
}

// nameAttr returns "CN=..." / "O=..." for want, or the DN's first attribute
// if it has none (the Flask app always took a fixed position, which landed
// on C= for DigiCert-style subjects that start with the country).
func nameAttr(n pkix.Name, want asn1.ObjectIdentifier) string {
	format := func(a pkix.AttributeTypeAndValue) string {
		k, ok := shortNames[a.Type.String()]
		if !ok {
			k = a.Type.String()
		}
		return fmt.Sprintf("%s=%v", k, a.Value)
	}
	for _, a := range n.Names {
		if a.Type.Equal(want) {
			return format(a)
		}
	}
	if len(n.Names) > 0 {
		return format(n.Names[0])
	}
	return ""
}

// subjectAltNames lists the SANs the way OpenSSL prints them ("DNS:x",
// "IP Address:1.2.3.4", "email:a@b", "URI:..."), sorted.
func subjectAltNames(c *x509.Certificate) []string {
	var out []string
	for _, d := range c.DNSNames {
		out = append(out, "DNS:"+d)
	}
	for _, ip := range c.IPAddresses {
		out = append(out, "IP Address:"+ip.String())
	}
	for _, e := range c.EmailAddresses {
		out = append(out, "email:"+e)
	}
	for _, u := range c.URIs {
		out = append(out, "URI:"+u.String())
	}
	sort.Strings(out)
	return out
}

// rdapFirstEntityName asks RDAP who holds ip and returns the vCard name
// (fn) of the first entity listed, like ipwhois's lookup_rdap()["objects"]
// did. ARIN redirects queries for other RIRs' space to the right server.
func rdapFirstEntityName(ctx context.Context, ip net.IP) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://rdap.arin.net/registry/ip/"+ip.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/rdap+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("rdap %s: %s", ip, resp.Status)
	}
	var body rdapResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("rdap %s: %w", ip, err)
	}
	for _, e := range body.Entities {
		if fn := e.fn(); fn != "" {
			return fn, nil
		}
	}
	return "", nil
}

type rdapResponse struct {
	Entities []rdapEntity `json:"entities"`
}

type rdapEntity struct {
	// ["vcard", [["version", {}, "text", "4.0"], ["fn", {}, "text", "Google LLC"], ...]]
	VCardArray []json.RawMessage `json:"vcardArray"`
}

func (e rdapEntity) fn() string {
	if len(e.VCardArray) < 2 {
		return ""
	}
	var props [][]json.RawMessage
	if json.Unmarshal(e.VCardArray[1], &props) != nil {
		return ""
	}
	for _, p := range props {
		var name, value string
		if len(p) >= 4 && json.Unmarshal(p[0], &name) == nil && name == "fn" && json.Unmarshal(p[3], &value) == nil {
			return value
		}
	}
	return ""
}
