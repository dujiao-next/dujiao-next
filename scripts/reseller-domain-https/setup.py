"""Provision HTTPS for a Dujiao-Next custom domain from a local socket request."""

import fcntl
import ipaddress
import json
import os
from pathlib import Path
import re
import secrets
import sqlite3
import subprocess
import sys
import tempfile
import time


APP_DB = Path(os.environ.get("DUJIAO_HTTPS_APP_DB", "/opt/dujiao-next/db/dujiao.db"))
WEBROOT = Path("/var/www/letsencrypt")
AVAILABLE = Path("/etc/nginx/sites-available")
ENABLED = Path("/etc/nginx/sites-enabled")
CERTBOT = os.environ.get("DUJIAO_HTTPS_CERTBOT", "/usr/bin/certbot")
NGINX = "/usr/sbin/nginx"
PROXY_TEMPLATE = Path(os.environ.get("DUJIAO_HTTPS_NGINX_TLS_TEMPLATE", "/etc/nginx/dujiao-reseller-tls.template"))
PUBLIC_CONFIG = "/api/v1/public/config"
DOMAIN_PATTERN = re.compile(r"(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]{0,61}[a-z0-9]")


def run(*command, timeout=6):
    return subprocess.check_output(command, text=True, stderr=subprocess.PIPE, timeout=timeout)


def domain_record(domain_id):
    with sqlite3.connect(APP_DB.as_uri() + "?mode=ro", timeout=2, uri=True) as database:
        return database.execute(
            """
            SELECT d.domain, d.status FROM reseller_domains d
            JOIN reseller_profiles p ON p.id = d.reseller_id
            WHERE d.id = ? AND d.type = 'custom' AND d.deleted_at IS NULL
              AND d.status IN ('pending_review', 'active')
              AND p.status = 'active' AND p.deleted_at IS NULL
            """,
            (domain_id,),
        ).fetchone()


def dns_matches(domain, expected_ip):
    for record_type, expected in (("A", {expected_ip}), ("AAAA", set())):
        answer = run("/usr/bin/dig", "+time=2", "+tries=1", "+noall", "+comments", "+answer",
                     "@1.1.1.1", domain, record_type, timeout=4)
        if not re.search(r"\bstatus: NOERROR,", answer):
            return False
        values = set(re.findall(rf"(?m)^\S+\s+\d+\s+IN\s+{record_type}\s+(\S+)", answer))
        if values != expected:
            return False
    return True


def fetch(domain, path, http=False):
    scheme, port = ("http", 80) if http else ("https", 443)
    output = run("/usr/bin/curl", "--noproxy", "*", "--silent", "--show-error",
                 "--max-time", "3", "--max-filesize", "262144",
                 "--resolve", f"{domain}:{port}:127.0.0.1", "--write-out", "\n%{http_code}",
                 f"{scheme}://{domain}{path}", timeout=4)
    body, _, status = output.rpartition("\n")
    return status, body


def https_ready(domain, domain_id):
    try:
        if fetch(domain, "/health")[0] != "200" or fetch(domain, "/api/v1/admin/login")[0] != "404":
            return False
        row = domain_record(domain_id)
        if row is None or row[0] != domain:
            return False
        status, body = fetch(domain, PUBLIC_CONFIG)
        payload = json.loads(body)
        if row[1] == "pending_review":
            return status == "404" and payload.get("message") == "site unavailable"
        tenant = payload.get("data", {}).get("tenant", {})
        return status == "200" and tenant.get("mode") == "reseller"
    except (subprocess.SubprocessError, ValueError, AttributeError):
        return False


def render_vhosts(domain, marker):
    tls = PROXY_TEMPLATE.read_text(encoding="utf-8")
    if len(re.findall(r"(?m)^\s*server\s*\{", tls)) != 1 or tls.count("@DOMAIN@") != 1 or tls.count("@CERT_DIR@") != 2:
        raise ValueError("Expected one TLS server with @DOMAIN@ and two @CERT_DIR@ placeholders")
    if not re.search(r"(?m)^\s*server_name\s+@DOMAIN@;", tls):
        raise ValueError("Expected a dedicated server_name for the custom domain")
    tls = tls.replace("@DOMAIN@", domain).replace("@CERT_DIR@", f"/etc/letsencrypt/live/{domain}")
    http = """server {
    listen 80;
    listen [::]:80;
    server_name @DOMAIN@;
    server_tokens off;
    location ^~ /.well-known/acme-challenge/ {
        root /var/www/letsencrypt;
        default_type text/plain;
        try_files $uri =404;
    }
    location / { @ACTION@ }
}
""".replace("@DOMAIN@", domain)
    bootstrap = marker + http.replace("@ACTION@", "return 404;")
    redirect = http.replace("@ACTION@", f"return 301 https://{domain}$request_uri;")
    return bootstrap, marker + redirect + "\n" + tls


def write_vhost(path, content):
    with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=AVAILABLE, delete=False) as output:
        temp = Path(output.name)
        try:
            os.fchmod(output.fileno(), 0o644)
            output.write(content)
            output.flush()
            os.fsync(output.fileno())
            os.replace(temp, path)
        finally:
            temp.unlink(missing_ok=True)


def reload_nginx():
    run(NGINX, "-t")
    run("/usr/bin/systemctl", "reload", "nginx")


def check_acme_path(domain):
    value = secrets.token_urlsafe(18)
    path = "/.well-known/acme-challenge/dujiao-preflight-" + value
    probe = WEBROOT / path.lstrip("/")
    try:
        probe.write_text(value)
        probe.chmod(0o644)
        for _ in range(8):
            if fetch(domain, path, http=True) == ("200", value):
                return
            time.sleep(0.3)
        raise RuntimeError("Nginx did not serve the HTTP validation file")
    finally:
        probe.unlink(missing_ok=True)


def configure(domain_id, domain):
    site = AVAILABLE / ("dujiao-reseller-" + str(domain_id))
    link = ENABLED / site.name
    marker = f"# Dujiao-Next reseller HTTPS {domain_id} {domain}\n"
    previous = site.read_text() if site.is_file() and not site.is_symlink() else None
    linked = os.path.lexists(link)
    if linked and (not link.is_symlink() or link.resolve() != site):
        return "conflict"
    if os.path.lexists(site) and (previous is None or not previous.startswith(marker)):
        return "conflict"

    names = re.findall(r"(?m)^\s*server_name\s+([^;]+);", run(NGINX, "-T"))
    named = any(domain in nameset.split() or "." + domain in nameset.split() for nameset in names)
    if named and https_ready(domain, domain_id):
        return "ready"
    if named and (previous is None or not linked):
        return "conflict"

    bootstrap, final = render_vhosts(domain, marker)
    try:
        write_vhost(site, bootstrap)
        if not linked:
            link.symlink_to(site)
        reload_nginx()
        check_acme_path(domain)
        run(CERTBOT, "certonly", "--webroot", "-w", str(WEBROOT), "--cert-name", domain,
            "-d", domain, "--non-interactive", "--agree-tos", "--keep-until-expiring", timeout=35)
        write_vhost(site, final)
        reload_nginx()
        for _ in range(8):
            if https_ready(domain, domain_id):
                return "ready"
            time.sleep(0.3)
        raise RuntimeError("The configured reseller HTTPS endpoint did not pass verification")
    except Exception:
        if not linked:
            link.unlink(missing_ok=True)
        if previous is None:
            site.unlink(missing_ok=True)
        else:
            write_vhost(site, previous)
        reload_nginx()
        raise


def handle_request():
    request = sys.stdin.buffer.readline(22)
    if not re.fullmatch(rb"[1-9][0-9]{0,18}\n", request):
        return "invalid"
    domain_id = int(request)
    if domain_id > 2**63 - 1:
        return "invalid"
    with open("/run/dujiao-reseller-https.lock", "a") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return "busy"
        row = domain_record(domain_id)
        if row is None:
            return "invalid"
        domain = row[0]
        if len(domain) > 253 or DOMAIN_PATTERN.fullmatch(domain) is None:
            return "invalid"
        expected_ip = str(ipaddress.IPv4Address(os.environ["DUJIAO_HTTPS_ORIGIN"]))
        if not dns_matches(domain, expected_ip):
            return "dns"
        return configure(domain_id, domain)


if __name__ == "__main__":
    os.umask(0o077)
    try:
        result = handle_request()
    except Exception as error:
        print(f"HTTPS setup failed: {error}", file=sys.stderr)
        if isinstance(error, subprocess.CalledProcessError):
            print((error.stderr or "")[:2000], file=sys.stderr)
        result = "failed"
    print(result, flush=True)
