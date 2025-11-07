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

Run the helper script to build the binary, install the `launchd` plist, and start the agent:

```sh
./scripts/install-launch-agent.sh
```

The agent is installed as `com.trixtur.gh-review-notifier`, runs at login, and restarts automatically if it crashes. Logs land in `~/Library/Logs/gh-review-notifier.log`.

To adjust the poll interval or flags, edit `~/Library/LaunchAgents/com.trixtur.gh-review-notifier.plist` and run `launchctl kickstart -k gui/$UID/com.trixtur.gh-review-notifier`.

## Cache

State is persisted in `~/Library/Application Support/gh-review-notifier/state.json` (or the system-config equivalent) and stores:
- Last seen timestamps for assigned PR updates
- Last seen comment/review timestamps for your authored PRs

Delete the cache file to resync from scratch if needed.
