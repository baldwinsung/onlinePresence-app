# onlinePresence-app

Flask REST API to look up the online presence of a domain — WHOIS registration, hosting provider, and SSL certificate details — all via HTTP PUT endpoints.

## Getting Started

### Docker (recommended)

```bash
docker buildx build -t onlinepresence-app .
docker run --name op-app -p 8000:5000 onlinepresence-app
```

### Local (Flask)

```bash
mkvirtualenv onlinePresence-app
pip install -r requirements.txt
flask run
```

List available routes:
```bash
flask routes
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
"MarkMonitor, Inc."

curl -X PUT http://localhost:8000/domainwhois_created/google.com
"Mon, 15 Sep 1997 04:00:00 GMT"

curl -X PUT http://localhost:8000/domainwhois_expires/google.com
"Thu, 14 Sep 2028 04:00:00 GMT"

curl -X PUT http://localhost:8000/sslcertificate_subject/google.com
"CN=*.google.com"

curl -X PUT http://localhost:8000/sslcertificate_issuer/google.com
"O=Google Trust Services LLC"

curl -X PUT http://localhost:8000/sslcertificate_notafter/google.com
"Mon, 29 Jul 2024 13:42:08 GMT"

curl -X PUT http://localhost:8000/sslcertificate_san/meta.com
[" DNS:meta.com","DNS:*.meta.com"]
```

## Credits

[Richard Penman](https://github.com/richardpenman) for python-whois
