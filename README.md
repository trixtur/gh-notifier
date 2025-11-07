# gh-review-notifier

A small Go daemon that polls GitHub via `gh` and notifies you about:
- Pull requests that request your review (with additions, deletions, files changed)
- New comments or reviews on pull requests you authored

## Prerequisites

- macOS (uses Notification Center via `osascript`)
- GitHub CLI (`gh`) installed and authenticated (`gh auth login`)
- Go 1.24+

## Running locally

```sh
go run ./cmd/gh-review-notifier
```

The first poll seeds the local cache without sending notifications so you are not spammed with existing activity. Subsequent changes trigger alerts.

### Flags

- `-interval` (default `3m`) — how often to poll GitHub.
- `-assigned-query` — search query for review requests. Defaults to `is:open is:pr archived:false user-review-requested:@me org:deseretdigital draft:false`.
- `-author` — GitHub login used to track your authored PRs. Defaults to the account returned by `gh auth status`.
- `-cache` — override the cache file location (defaults to `~/Library/Application Support/gh-review-notifier/state.json` on macOS).

## Launch agent (optional)

To keep the notifier running, consider adding a `launchd` plist that runs `go run` or the compiled binary at login. Example snippet:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>com.example.gh-review-notifier</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/gh-review-notifier</string>
        <string>-interval</string><string>2m</string>
    </array>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
</dict>
</plist>
```

Adjust the binary path and options to match your setup, then load with `launchctl load ~/Library/LaunchAgents/com.example.gh-review-notifier.plist`.

## Cache

State is persisted in `~/Library/Application Support/gh-review-notifier/state.json` (or the system-config equivalent) and stores:
- Last seen timestamps for assigned PR updates
- Last seen comment/review timestamps for your authored PRs

Delete the cache file to resync from scratch if needed.
