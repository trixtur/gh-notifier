package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"gh-review-notifier/internal/cache"
	"gh-review-notifier/internal/github"
	"gh-review-notifier/internal/monitor"
	"gh-review-notifier/internal/notify"
)

const defaultAssignedQuery = "is:open is:pr archived:false user-review-requested:@me org:deseretdigital draft:false"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	pollInterval := flag.Duration("interval", 3*time.Minute, "poll interval for checking GitHub")
	assignedQuery := flag.String("assigned-query", defaultAssignedQuery, "GitHub search query for review requests")
	author := flag.String("author", "", "GitHub username for authored PR tracking (defaults to authenticated user)")
	cacheFile := flag.String("cache", "", "path to cache file (defaults to system config dir)")
	flag.Parse()

	client := github.NewClient(logger)

	var err error
	if *author == "" {
		*author, err = client.CurrentUserLogin(ctx)
		if err != nil {
			logger.Error("failed to resolve current GitHub user", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	if *cacheFile == "" {
		*cacheFile, err = defaultCachePath()
		if err != nil {
			logger.Error("failed to determine default cache path", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	state, err := cache.Load(*cacheFile)
	if err != nil {
		logger.Error("failed to load cache", slog.String("error", err.Error()))
		os.Exit(1)
	}

	mon := monitor.NewMonitor(monitor.Config{
		PollInterval:  *pollInterval,
		AssignedQuery: *assignedQuery,
		Author:        *author,
		CacheFile:     *cacheFile,
	}, client, notify.NewNotifier(logger), state, logger)

	logger.Info("starting gh-review-notifier",
		slog.Duration("interval", *pollInterval),
		slog.String("author", *author),
		slog.String("cache", *cacheFile),
	)

	if err := mon.Run(ctx); err != nil {
		logger.Error("monitor stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func defaultCachePath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	return filepath.Join(cfgDir, "gh-review-notifier", "state.json"), nil
}
