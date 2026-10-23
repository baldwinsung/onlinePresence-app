# onlinePresence-app

Go REST API (Flask until 2026-10-23) that exposes domain WHOIS info, hosting provider lookup, and SSL certificate details via HTTP PUT endpoints.

## Commands

### Run locally
```bash
go test ./...
go build -o bin/onlinepresence-server ./cmd/onlinepresence-server
LISTEN_ADDR=:8000 bin/onlinepresence-server serve   # default :8080
```

### Run in Docker
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/onlinepresence-server ./cmd/onlinepresence-server
docker buildx build -t app .
docker run --name test -p 8000:8080 app
```

WHOIS (port 43) and TLS (443) lookups dial out directly; they ignore
`HTTPS_PROXY`, which only the RDAP lookup honors.

## API Endpoints

All endpoints use `PUT` with the domain as a path parameter.

| Endpoint | Returns |
|---|---|
| `/domainwhois_registrar/<domain>` | Registrar name |
| `/domainwhois_created/<domain>` | Registration date |
| `/domainwhois_expires/<domain>` | Expiration date |
| `/hostingprovider/<domain>` | Hosting provider from RDAP A-record lookup |
| `/sslcertificate_subject/<domain>` | Certificate subject (CN) |
| `/sslcertificate_issuer/<domain>` | Certificate issuer (O) |
| `/sslcertificate_notbefore/<domain>` | Certificate valid-from date |
| `/sslcertificate_notafter/<domain>` | Certificate expiry date |
| `/sslcertificate_san/<domain>` | Subject Alternative Names (sorted list) |

## Architecture

One Go binary, `cmd/onlinepresence-server/main.go`. No database. All lookups are live network calls — WHOIS via `likexian/whois` + `whois-parser`, DNS via the Go resolver, RDAP as a plain JSON GET to `rdap.arin.net` (redirects to the right RIR; first entity's vCard `fn`), TLS via `crypto/tls` (unverified, like the old `ssl.get_server_certificate`). Outbound calls sit behind the `lookups` struct so tests can fake them. See README's "Differences from the Flask version".

## Notes

- SSL endpoints connect on port 443; they will time out if the domain blocks that port.
- `hostingprovider` returns the first RDAP entity name for the domain's first A record — may not always be the actual hosting provider.
- All dates are returned as JSON strings in HTTP date format.
