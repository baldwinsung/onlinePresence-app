from flask import Flask, jsonify
from datetime import datetime
import ssl
import dns.resolver
import OpenSSL
import whois
from ipwhois import IPWhois

app = Flask(__name__)


def _load_ssl_cert(domain):
    c = ssl.get_server_certificate((domain, 443), timeout=5)
    return OpenSSL.crypto.load_certificate(OpenSSL.crypto.FILETYPE_PEM, c)


def _error(msg, status=400):
    return jsonify({"error": msg}), status


@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "ok"})


@app.route('/', methods=['GET'])
def main_page():
    return jsonify("main page, enter the target path")


@app.route('/domainwhois_registrar/<domain>', methods=['PUT'])
def domainWhois_registrar(domain):
    try:
        w = whois.whois(domain)
        return jsonify(w.registrar)
    except Exception as e:
        return _error(str(e), 502)


@app.route('/domainwhois_created/<domain>', methods=['PUT'])
def domainWhois_created(domain):
    try:
        w = whois.whois(domain)
        date = w.creation_date[0] if isinstance(w.creation_date, list) else w.creation_date
        return jsonify(date)
    except Exception as e:
        return _error(str(e), 502)


@app.route('/domainwhois_expires/<domain>', methods=['PUT'])
def domainWhois_expires(domain):
    try:
        w = whois.whois(domain)
        date = w.expiration_date[0] if isinstance(w.expiration_date, list) else w.expiration_date
        return jsonify(date)
    except Exception as e:
        return _error(str(e), 502)


@app.route('/hostingprovider/<domain>', methods=['PUT'])
def hostingProvider(domain):
    try:
        result = dns.resolver.resolve(domain, 'A')
        ips = [r.to_text() for r in result]
        if not ips:
            return _error("No A record found", 404)
        lookup = IPWhois(ips[0])
        p = lookup.lookup_rdap()
        if p['objects']:
            for x in p['objects']:
                name = p['objects'][x]['contact']['name']
                return jsonify(name)
        return _error("Hosting provider not found", 404)
    except Exception as e:
        return _error(str(e), 502)


@app.route('/sslcertificate_subject/<domain>', methods=['PUT'])
def sslCertificate_subject(domain):
    try:
        x = _load_ssl_cert(domain)
        cn = x.get_subject()
        cn_str = "".join(f"/{name.decode()}={value.decode()}" for name, value in cn.get_components())
        return jsonify(cn_str.split('/')[1])
    except Exception as e:
        return _error(str(e), 502)


@app.route('/sslcertificate_issuer/<domain>', methods=['PUT'])
def sslCertificate_issuer(domain):
    try:
        x = _load_ssl_cert(domain)
        ir = x.get_issuer()
        ir_str = "".join(f"/{name.decode()}={value.decode()}" for name, value in ir.get_components())
        return jsonify(ir_str.split('/')[2])
    except Exception as e:
        return _error(str(e), 502)


@app.route('/sslcertificate_notbefore/<domain>', methods=['PUT'])
def sslCertificate_notbefore(domain):
    try:
        x = _load_ssl_cert(domain)
        nb = datetime.strptime(x.get_notBefore().decode('ascii'), '%Y%m%d%H%M%SZ')
        return jsonify(nb)
    except Exception as e:
        return _error(str(e), 502)


@app.route('/sslcertificate_notafter/<domain>', methods=['PUT'])
def sslCertificate_notafter(domain):
    try:
        x = _load_ssl_cert(domain)
        na = datetime.strptime(x.get_notAfter().decode('ascii'), '%Y%m%d%H%M%SZ')
        return jsonify(na)
    except Exception as e:
        return _error(str(e), 502)


@app.route('/sslcertificate_san/<domain>', methods=['PUT'])
def sslCertificate_san(domain):
    try:
        x = _load_ssl_cert(domain)
        ec = x.get_extension_count()
        for i in range(ec):
            ge = x.get_extension(i)
            if 'subjectAltName' in str(ge.get_short_name()):
                return jsonify(sorted(ge.__str__().split(',')))
        return _error("No SAN extension found", 404)
    except Exception as e:
        return _error(str(e), 502)


if __name__ == '__main__':
    app.run(port=5000)
