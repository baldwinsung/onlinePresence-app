# onlinePresence-app

Flask REST API that exposes domain WHOIS info, hosting provider lookup, and SSL certificate details via HTTP PUT endpoints.

## Commands

### Run locally
```bash
mkvirtualenv onlinePresence-app
pip install -r requirements.txt
flask run          # http://localhost:5000
flask routes       # list all endpoints
```

### Run in Docker
```bash
docker buildx build -t app .
docker run --name test -p 8000:5000 app
```

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

Single-file Flask app (`app.py`). No database. All lookups are live network calls — WHOIS via `python-whois`, DNS/IP via `dnspython` + `ipwhois`, SSL via stdlib `ssl` + `pyOpenSSL`.

## Notes

- SSL endpoints connect on port 443; they will time out if the domain blocks that port.
- `hostingprovider` returns the first RDAP contact name from the IP's ASN — may not always be the actual hosting provider.
- All dates are returned as JSON strings in HTTP date format.
