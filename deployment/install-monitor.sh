#!/usr/bin/env bash
set -euo pipefail
[[ $EUID -eq 0 ]] || { echo 'Run as root (sudo bash deployment/install-monitor.sh)' >&2; exit 1; }
package_dir=$(cd "${1:-$(dirname "$0")/..}" && pwd)
[[ -f "$package_dir/pulse-monitor" && -f "$package_dir/out/index.html" ]] || { echo 'Incomplete release package' >&2; exit 1; }
addr=${PULSE_ADDR:-127.0.0.1:8080}
public_url=${PULSE_PUBLIC_URL:-}
cert=${PULSE_TLS_CERT:-}
key=${PULSE_TLS_KEY:-}
for argument in "$addr" "$public_url" "$cert" "$key"; do
 [[ "$argument" != *'"'* && "$argument" != *$'\n'* && "$argument" != *$'\r'* && "$argument" != *'%'* && "$argument" != *'\'* ]] || { echo 'Invalid systemd argument' >&2; exit 1; }
done
[[ -z "$cert" && -z "$key" || -f "$cert" && -f "$key" ]] || { echo 'Provide existing certificate and key together' >&2; exit 1; }
id pulse-monitor >/dev/null 2>&1 || useradd --system --home-dir /var/lib/pulse-monitor --shell /usr/sbin/nologin pulse-monitor
install -d -m 0755 /opt/pulse-monitor
install -d -m 0700 -o pulse-monitor -g pulse-monitor /var/lib/pulse-monitor
systemctl stop pulse-monitor.service 2>/dev/null || true
install -m 0755 "$package_dir/pulse-monitor" /opt/pulse-monitor/pulse-monitor
# Stage static files on the same filesystem; previous exports remain available for recovery.
stage=$(mktemp -d /opt/pulse-monitor/export.XXXXXXXX)
cp -R "$package_dir/out/." "$stage/"
chmod -R a+rX "$stage"
if [[ -e /opt/pulse-monitor/out ]]; then
 [[ -d /opt/pulse-monitor/out && ! -L /opt/pulse-monitor/out ]] || { echo 'Unexpected static path' >&2; exit 1; }
 mv /opt/pulse-monitor/out "/opt/pulse-monitor/out.previous.$(date +%s).$$"
fi
mv "$stage" /opt/pulse-monitor/out
arguments="-addr \"$addr\" -data /var/lib/pulse-monitor -static /opt/pulse-monitor/out"
[[ -z "$cert" ]] || arguments+=" -tls-cert \"$cert\" -tls-key \"$key\""
[[ -z "$public_url" ]] || arguments+=" -public-url \"$public_url\""
cat > /etc/systemd/system/pulse-monitor.service <<UNIT
[Unit]
Description=Pulse monitoring panel
After=network-online.target
Wants=network-online.target
[Service]
User=pulse-monitor
Group=pulse-monitor
WorkingDirectory=/opt/pulse-monitor
ExecStart=/opt/pulse-monitor/pulse-monitor $arguments
Restart=on-failure
RestartSec=5
UMask=0077
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/var/lib/pulse-monitor
TimeoutStopSec=20
[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable --now pulse-monitor.service
systemctl --no-pager status pulse-monitor.service