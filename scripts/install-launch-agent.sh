#!/usr/bin/env bash
set -euo pipefail

APP_ID="com.trixtur.gh-review-notifier"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="$REPO_ROOT/bin"
BINARY="$BIN_DIR/gh-review-notifier"
LAUNCH_AGENT_DIR="$HOME/Library/LaunchAgents"
PLIST_PATH="$LAUNCH_AGENT_DIR/${APP_ID}.plist"
LOG_DIR="$HOME/Library/Logs"
LOG_PATH="$LOG_DIR/gh-review-notifier.log"

echo "[gh-notifier] Building binary..."
mkdir -p "$BIN_DIR"
GOFLAGS=${GOFLAGS:-} go build -o "$BINARY" "$REPO_ROOT/cmd/gh-review-notifier"

mkdir -p "$LAUNCH_AGENT_DIR"
mkdir -p "$LOG_DIR"

cat >"$PLIST_PATH" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${APP_ID}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${BINARY}</string>
        <string>-interval</string>
        <string>2m</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
    <key>StandardOutPath</key>
    <string>${LOG_PATH}</string>
    <key>StandardErrorPath</key>
    <string>${LOG_PATH}</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>WorkingDirectory</key>
    <string>${REPO_ROOT}</string>
</dict>
</plist>
PLIST

echo "[gh-notifier] Loading launch agent..."
launchctl unload "$PLIST_PATH" >/dev/null 2>&1 || true
launchctl load "$PLIST_PATH"
launchctl start "$APP_ID"

echo "[gh-notifier] Installed launch agent at $PLIST_PATH"
echo "[gh-notifier] Logs will be written to $LOG_PATH"
