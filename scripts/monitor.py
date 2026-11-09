#!/usr/bin/env python3
"""Folio 生产健康检查：应用、容器、磁盘、内存、备份和 HTTPS 证书。

异常时通过 .env 里的 SMTP 配置发邮件到 FOLIO_ALERT_EMAIL。
只使用 Python 标准库，不依赖第三方包。
"""

import glob
import json
import os
import shutil
import smtplib
import socket
import ssl
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from email.message import EmailMessage
from email.utils import formataddr
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ENV_PATH = ROOT / ".env"
STATE_DIR = Path("/var/lib/folio-monitor")
STATE_PATH = STATE_DIR / "state.json"
ALERT_INTERVAL = 6 * 3600
ENV = {}


def load_env():
    values = {}
    if not ENV_PATH.exists():
        return values
    for line in ENV_PATH.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
            value = value[1:-1]
        values[key.strip()] = value
    return values


def env(key, default=""):
    return os.environ.get(key, ENV.get(key, default)).strip()


def run(cmd, timeout=15):
    return subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)


def check_http(url, label):
    try:
        request = urllib.request.Request(url, headers={"User-Agent": "folio-monitor/1.0"})
        with urllib.request.urlopen(request, timeout=12, context=ssl.create_default_context()) as response:
            body = response.read(8192).decode("utf-8", "replace")
            if response.status != 200:
                return False, f"{label} HTTP {response.status}"
            data = json.loads(body)
            if not data.get("ok"):
                return False, f"{label} 返回 ok=false: {body[:160]}"
            return True, f"{label} 正常"
    except urllib.error.HTTPError as exc:
        return False, f"{label} HTTP {exc.code}"
    except Exception as exc:
        return False, f"{label} 访问失败: {exc}"


def check_container(service):
    try:
        listed = run(
            ["docker", "ps", "--filter", f"label=com.docker.compose.service={service}", "--format", "{{.Names}}"]
        ).stdout.strip()
        if not listed:
            return False, f"{service} 容器不存在"
        name = listed.splitlines()[0]
        detail = run(
            ["docker", "inspect", "-f", "{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{end}}", name]
        ).stdout.strip()
        status, _, health = detail.partition("|")
        if status != "running":
            return False, f"{service} 状态 {status}"
        if health and health != "healthy":
            return False, f"{service} 健康状态 {health}"
        return True, f"{service} 正常运行"
    except Exception as exc:
        return False, f"{service} 检查失败: {exc}"


def check_disk():
    usage = shutil.disk_usage("/")
    used_percent = usage.used / usage.total * 100
    free_gb = usage.free / (1024 ** 3)
    if used_percent >= 85 or free_gb < 3:
        return False, f"磁盘使用 {used_percent:.1f}%，剩余 {free_gb:.1f}G"
    return True, f"磁盘使用 {used_percent:.1f}%，剩余 {free_gb:.1f}G"


def check_memory():
    values = {}
    for line in Path("/proc/meminfo").read_text(encoding="utf-8").splitlines():
        key, value = line.split(":", 1)
        values[key] = int(value.strip().split()[0])
    available_mb = values.get("MemAvailable", 0) / 1024
    if available_mb < 100:
        return False, f"可用内存 {available_mb:.0f}MB"
    return True, f"可用内存 {available_mb:.0f}MB"


def check_backup():
    files = sorted(glob.glob(str(ROOT / "backups" / "folio-*.tar.gz")), key=os.path.getmtime)
    if not files:
        return False, "没有找到备份文件"
    age_hours = (time.time() - os.path.getmtime(files[-1])) / 3600
    if age_hours > 36:
        return False, f"最近备份在 {age_hours:.1f} 小时前"
    return True, f"最近备份在 {age_hours:.1f} 小时前"


def check_offsite():
    if not env("OSS_BUCKET"):
        return True, "未启用异地备份"
    marker = Path("/var/lib/folio-offsite/last-success")
    if not marker.exists():
        return False, "尚未成功上传过异地备份"
    try:
        data = json.loads(marker.read_text(encoding="utf-8"))
        timestamp = float(data.get("timestamp", 0))
    except Exception:
        timestamp = marker.stat().st_mtime
    age_hours = (time.time() - timestamp) / 3600
    if age_hours > 36:
        return False, f"最近异地备份在 {age_hours:.1f} 小时前"
    return True, f"最近异地备份在 {age_hours:.1f} 小时前"


def check_cert(domain):
    if not domain:
        return True, "未配置域名，跳过证书检查"
    try:
        handshake = subprocess.run(
            ["openssl", "s_client", "-servername", domain, "-connect", f"{domain}:443"],
            input=b"",
            capture_output=True,
            timeout=15,
        )
        check = subprocess.run(
            ["openssl", "x509", "-noout", "-checkend", "1209600"],
            input=handshake.stdout,
            capture_output=True,
            timeout=15,
        )
        if check.returncode == 0:
            return True, "证书 14 天内不会过期"
        return False, "HTTPS 证书将在 14 天内过期或已过期"
    except Exception as exc:
        return False, f"证书检查失败: {exc}"


def send_email(subject, body):
    host = env("FOLIO_SMTP_HOST")
    if not host:
        print("[monitor] 未配置 SMTP，跳过邮件")
        return False
    port = int(env("FOLIO_SMTP_PORT", "465") or "465")
    user = env("FOLIO_SMTP_USER")
    password = env("FOLIO_SMTP_PASSWORD")
    sender = env("FOLIO_SMTP_FROM") or user
    recipient = env("FOLIO_ALERT_EMAIL") or sender
    if not sender or not recipient:
        print("[monitor] 未配置 FOLIO_ALERT_EMAIL / FOLIO_SMTP_FROM")
        return False
    message = EmailMessage()
    message["Subject"] = subject
    message["From"] = formataddr((env("FOLIO_SMTP_FROM_NAME", "知微"), sender))
    message["To"] = recipient
    message.set_content(body)
    tls = env("FOLIO_SMTP_TLS", "").lower() in ("1", "true", "yes", "on")
    try:
        if port == 465 or tls:
            server = smtplib.SMTP_SSL(host, port, timeout=20, context=ssl.create_default_context())
        else:
            server = smtplib.SMTP(host, port, timeout=20)
            server.ehlo()
            server.starttls(context=ssl.create_default_context())
            server.ehlo()
        try:
            if user:
                server.login(user, password)
            server.send_message(message)
        finally:
            server.quit()
        return True
    except Exception as exc:
        print(f"[monitor] 发送告警邮件失败: {exc}")
        return False


def load_state():
    try:
        return json.loads(STATE_PATH.read_text(encoding="utf-8"))
    except Exception:
        return {"checks": {}}


def save_state(state):
    STATE_DIR.mkdir(parents=True, exist_ok=True)
    STATE_PATH.write_text(json.dumps(state, ensure_ascii=False, indent=2), encoding="utf-8")


def main():
    global ENV
    ENV = load_env()
    if "--test-email" in sys.argv:
        ok = send_email(
            "[知微监控] 测试邮件",
            f"这是一封来自 {socket.gethostname()} 的测试邮件。\n时间：{time.strftime('%Y-%m-%d %H:%M:%S')}",
        )
        print("test email sent" if ok else "test email failed")
        return 0 if ok else 1

    domain = env("FOLIO_DOMAIN")
    if not domain:
        domain = urllib.parse.urlparse(env("FOLIO_SITE_URL")).hostname or ""
    checks = [
        ("app_health", lambda: check_http("http://127.0.0.1:8080/api/health", "应用")),
        (
            "https_health",
            lambda: check_http(f"https://{domain}/api/health", "公网 HTTPS")
            if domain
            else (True, "未配置域名，跳过公网检查"),
        ),
        ("container_app", lambda: check_container("app")),
        ("container_db", lambda: check_container("db")),
        ("container_redis", lambda: check_container("redis")),
        ("container_caddy", lambda: check_container("caddy")),
        ("disk", check_disk),
        ("memory", check_memory),
        ("backup", check_backup),
        ("offsite_backup", check_offsite),
        ("certificate", lambda: check_cert(domain)),
    ]

    state = load_state()
    now = time.time()
    alerts = []
    recoveries = []
    for name, check in checks:
        ok, detail = check()
        previous = state["checks"].get(name, {})
        print(f"[{'OK' if ok else 'FAIL'}] {name}: {detail}")
        if ok:
            if previous.get("ok") is False:
                recoveries.append(f"{name}: {detail}")
            state["checks"][name] = {"ok": True, "detail": detail}
            continue
        last_alert = previous.get("last_alert", 0)
        if previous.get("ok", True) or now - last_alert >= ALERT_INTERVAL:
            alerts.append(f"{name}: {detail}")
        state["checks"][name] = {"ok": False, "detail": detail, "last_alert": last_alert}

    if alerts:
        body = (
            f"主机：{socket.gethostname()}\n"
            f"时间：{time.strftime('%Y-%m-%d %H:%M:%S')}\n\n"
            "异常：\n"
            + "\n".join(f"- {item}" for item in alerts)
            + "\n\n请登录服务器检查：\n"
            "cd /opt/folio && docker compose -f docker-compose.yml -f docker-compose.edge.yml ps\n"
        )
        if send_email("[知微监控] 发现异常", body):
            for name, _ in checks:
                if any(item.startswith(name + ":") for item in alerts):
                    state["checks"][name]["last_alert"] = now

    if recoveries:
        body = (
            f"主机：{socket.gethostname()}\n"
            f"时间：{time.strftime('%Y-%m-%d %H:%M:%S')}\n\n"
            "已恢复：\n" + "\n".join(f"- {item}" for item in recoveries)
        )
        send_email("[知微监控] 已恢复", body)

    save_state(state)
    return 0


if __name__ == "__main__":
    sys.exit(main())
