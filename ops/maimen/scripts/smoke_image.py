#!/usr/bin/env python3
"""Run a newly built image with disposable data and no network connectivity."""

import argparse
import json
from pathlib import Path
import re
import secrets
import subprocess
import tempfile
import time
from urllib.parse import urljoin, urlsplit
import uuid


def docker(*args, check=True, timeout=30):
    return subprocess.run(
        ["docker", *args], check=check, timeout=timeout,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
        encoding="utf-8", errors="replace",
    )


def get(container, route):
    return docker(
        "exec", container, "wget", "-q", "-O", "-",
        "http://127.0.0.1:8080" + route, check=False, timeout=10,
    )


def check_spa(container, route, admin=False):
    page = get(container, route)
    if page.returncode or "<html" not in page.stdout.lower():
        raise RuntimeError("Embedded SPA unavailable at " + route)
    if "__DJ_ADMIN_BASE__" in page.stdout:
        raise RuntimeError("Admin path placeholder was not replaced")
    scripts = re.findall(r'<script\b[^>]*\bsrc=["\']([^"\']+)["\']', page.stdout)
    modules = [value for value in scripts if ".js" in value]
    if not modules:
        raise RuntimeError("No embedded JavaScript entry at " + route)
    parsed = urlsplit(urljoin("http://127.0.0.1:8080" + route, modules[0]))
    if parsed.scheme != "http" or parsed.netloc != "127.0.0.1:8080":
        raise RuntimeError("Entry script unexpectedly depends on another origin")
    if admin and not parsed.path.startswith("/verify-admin/"):
        raise RuntimeError("Admin assets do not follow the configured path")
    resource = parsed.path + ("?" + parsed.query if parsed.query else "")
    asset = get(container, resource)
    if asset.returncode or not asset.stdout.strip() or "<html" in asset.stdout[:200].lower():
        raise RuntimeError("Embedded JavaScript asset unavailable at " + resource)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("image", help="Locally built fullstack image; no automatic pull")
    args = parser.parse_args()
    container = "maimen-image-smoke-" + uuid.uuid4().hex[:12]
    config = {
        "app": {"secret_key": secrets.token_hex(32)},
        "server": {"host": "0.0.0.0", "port": 8080, "mode": "release", "trusted_proxies": []},
        "database": {"driver": "sqlite", "dsn": "/app/db/smoke.db"},
        "jwt": {"secret": secrets.token_hex(32)},
        "user_jwt": {"secret": secrets.token_hex(32)},
        "bootstrap": {"default_admin_username": "", "default_admin_password": ""},
        "redis": {"enabled": False},
        "queue": {"enabled": False},
        "email": {"enabled": False},
        "telegram_auth": {"enabled": False},
        "google_auth": {"enabled": False},
        "reseller": {"enabled": False},
        "web": {"admin_path": "/verify-admin"},
    }
    with tempfile.TemporaryDirectory(prefix="maimen-image-smoke-") as directory:
        config_path = Path(directory) / "config.yml"
        config_path.write_text(json.dumps(config), encoding="utf-8")
        config_path.chmod(0o600)
        created = False
        try:
            docker(
                "run", "--detach", "--pull", "never", "--platform", "linux/amd64",
                "--name", container, "--network", "none", "--read-only",
                "--memory", "512m", "--cpus", "1",
                "--mount", "type=bind,src=" + str(config_path) + ",dst=/app/config.yml,readonly",
                "--tmpfs", "/app/db:rw,nosuid,size=64m",
                "--tmpfs", "/app/logs:rw,nosuid,size=32m",
                "--tmpfs", "/app/uploads:rw,nosuid,size=16m",
                args.image, "./dujiao-next", "-mode", "api",
            )
            created = True
            deadline = time.monotonic() + 90
            while True:
                health = get(container, "/health")
                if health.returncode == 0:
                    break
                running = docker("inspect", "--format", "{{.State.Running}}", container)
                if running.stdout.strip() != "true" or time.monotonic() >= deadline:
                    raise RuntimeError("Image did not become healthy")
                time.sleep(1)
            if json.loads(health.stdout).get("status") != "ok":
                raise RuntimeError("Unexpected health response")
            check_spa(container, "/")
            check_spa(container, "/verify-admin/", admin=True)
            # Exercise the shipped operator binary against this disposable DB.
            # status never migrates or resumes payment automation.
            status = docker("exec", container, "/app/payment-ops", "-action", "status")
            control = json.loads(status.stdout.strip().splitlines()[-1])
            if control.get("Frozen") is not True or control.get("Revision") != 1:
                raise RuntimeError("Fresh image did not preserve default payment freeze")
            print("Image smoke test passed: API, storefront, custom admin path, assets, and payment-ops default freeze.")
        except Exception:
            if created:
                logs = docker("logs", "--tail", "80", container, check=False)
                print(logs.stdout + logs.stderr)
            raise
        finally:
            if created:
                docker("rm", "--force", container, check=False)


if __name__ == "__main__":
    main()
