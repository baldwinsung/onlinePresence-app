# onlinePresence-app

Go REST API to look up the online presence of a domain — WHOIS registration, hosting provider, and SSL certificate details — all via HTTP PUT endpoints. (A Flask app until 2026-10-23; same routes and JSON.)

## Getting Started

### Local

```bash
go test ./...
go build -o bin/onlinepresence-server ./cmd/onlinepresence-server
LISTEN_ADDR=:8000 bin/onlinepresence-server serve
```

### Docker

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/onlinepresence-server ./cmd/onlinepresence-server
docker buildx build -t onlinepresence-app .
docker run --name op-app -p 8000:8080 onlinepresence-app
```

## API Reference

All endpoints use `PUT`. Replace `<domain>` with a domain name (e.g. `google.com`).

| Endpoint | Description |
|---|---|
| `PUT /domainwhois_registrar/<domain>` | Domain registrar |
| `PUT /domainwhois_created/<domain>` | Domain registration date |
| `PUT /domainwhois_expires/<domain>` | Domain expiration date |
| `PUT /hostingprovider/<domain>` | Hosting provider (via IP RDAP lookup) |
| `PUT /sslcertificate_subject/<domain>` | SSL certificate subject (CN) |
| `PUT /sslcertificate_issuer/<domain>` | SSL certificate issuer (O) |
| `PUT /sslcertificate_notbefore/<domain>` | SSL valid-from date |
| `PUT /sslcertificate_notafter/<domain>` | SSL expiry date |
| `PUT /sslcertificate_san/<domain>` | Subject Alternative Names (sorted) |

## Examples

```bash
curl -X PUT http://localhost:8000/domainwhois_registrar/google.com
"MarkMonitor Inc."

curl -X PUT http://localhost:8000/domainwhois_created/google.com
"Mon, 15 Sep 1997 04:00:00 GMT"

curl -X PUT http://localhost:8000/domainwhois_expires/google.com
"Thu, 14 Sep 2028 04:00:00 GMT"

curl -X PUT http://localhost:8000/sslcertificate_subject/google.com
"CN=*.google.com"

curl -X PUT http://localhost:8000/sslcertificate_issuer/google.com
"O=Google Trust Services"

curl -X PUT http://localhost:8000/sslcertificate_notafter/google.com
"Mon, 29 Jul 2024 13:42:08 GMT"

curl -X PUT http://localhost:8000/sslcertificate_san/meta.com
["DNS:*.meta.com","DNS:meta.com"]
```

### Differences from the Flask version (2026-10-23)

- `sslcertificate_subject` / `_issuer` return the `CN=` / `O=` attribute.
  Flask returned a fixed position in the DN, so certificates whose subject
  starts with the country (DigiCert's, e.g. meta.com) gave `"C=US"`.
- `sslcertificate_san` works again (it returned 502 for every domain under
  pyOpenSSL 26) and no longer has a stray leading space on all but the
  first entry.
- WHOIS comes from `likexian/whois` + `whois-parser`, which takes the
  registry's registrar name (`"MarkMonitor Inc."`) where python-whois took
  the registrar's own (`"MarkMonitor, Inc."`).
- Errors are always JSON (`{"error": ...}`); unknown routes and wrong
  methods return Go's plain-text 404/405 instead of Flask's HTML pages.

## Deployment

Live at `op.baldwinsung.com`, publicly accessible (no auth — this is a
public lookup API, unlike the private apps in this fleet). Runs as 1
replica on each of 3 independent VPS (NY, LA, Taipei — see
`personal_vps`), each its own single-node k3s cluster. No database, no
persistent volume, no secrets of its own at all — every endpoint is a
live outbound network call, so there's nothing to keep in sync between
replicas and nothing app-specific to configure.

```bash
# One-time per VPS, after the image is built+imported (see below):
kubectl apply -f deploy/k8s/vps/namespace.yaml -f deploy/k8s/vps/app.yaml
```

Routing is one `ingress_rule` in `baldwinsung_cloudflare/zerotrust/tunnel.tf`'s
shared `vps_shared` Cloudflare Tunnel (`op.baldwinsung.com` →
`onlinepresence-app.onlinepresence-app.svc.cluster.local:5000`), fronted
by `personal_vps`'s single shared `cloudflared` Deployment per VPS
(`personal_vps/deploy/k8s/cloudflared-shared/`) — **not** a dedicated
tunnel/Deployment of this app's own. This app briefly had one (created
alongside the VPS migration, 2026-09-26), then it was folded into the
shared tunnel the same day after 8 separate per-app cloudflared processes
were found driving a multi-hour crash-loop incident on the LA VPS — see
`personal_vps/CLAUDE.md`'s "Shared cloudflared" section for the full
story. Adding this (or any) app to the fleet now means adding an
`ingress_rule` to `vps_shared`'s config, not standing up a new tunnel.

No CI is wired up yet — build the binary (see Getting Started), then the
image with `docker buildx build -t localhost/onlinepresence-app:vps-latest
--load .` and distribute it to each VPS with `docker save` + `k3s ctr -n
k8s.io images import`, matching `imagePullPolicy: Never` in `app.yaml`.
The container listens on 8080; the Service keeps port 5000 so the tunnel
rule is unchanged. Migrated
2026-09-26 off an old single-point deployment (podman + a separate system
containerd + a systemd unit, LA only, no redundancy) that predated this
fleet pattern.

## Credits

[Richard Penman](https://github.com/richardpenman) for python-whois (used
until 2026-10-23), [Li Kexian](https://github.com/likexian) for
`whois` and `whois-parser`
