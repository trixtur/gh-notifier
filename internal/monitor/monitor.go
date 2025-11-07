package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gh-review-notifier/internal/cache"
	githubapi "gh-review-notifier/internal/github"
	"gh-review-notifier/internal/notify"
)

type Config struct {
	PollInterval  time.Duration
	AssignedQuery string
	Author        string
	CacheFile     string
	MaxResults    int
}

type GitHubClient interface {
	SearchAssignedPullRequests(ctx context.Context, query string, limit int) ([]githubapi.PullRequestSummary, error)
	ListAuthoredPullRequests(ctx context.Context, author string, limit int) ([]githubapi.PullRequestSummary, error)
	PullRequestDetails(ctx context.Context, repo string, number int) (*githubapi.PullRequest, error)
	IssueCommentsSince(ctx context.Context, repo string, number int, since time.Time) ([]githubapi.IssueComment, error)
	Reviews(ctx context.Context, repo string, number int) ([]githubapi.Review, error)
}

type Monitor struct {
	cfg      Config
	client   GitHubClient
	notifier notify.Notifier
	state    *cache.State
	logger   *slog.Logger
	mu       sync.Mutex
}

const defaultMaxResults = 30

func NewMonitor(cfg Config, client GitHubClient, notifier notify.Notifier, state *cache.State, logger *slog.Logger) *Monitor {
	if cfg.MaxResults == 0 {
		cfg.MaxResults = defaultMaxResults
	}
	return &Monitor{
		cfg:      cfg,
		client:   client,
		notifier: notifier,
		state:    state,
		logger:   logger,
	}
}

func (m *Monitor) Run(ctx context.Context) error {
	ticker := time.NewTicker(m.cfg.PollInterval)
	defer ticker.Stop()

	if err := m.poll(ctx); err != nil {
		m.logger.Warn("initial poll failed", slog.String("error", err.Error()))
	}
	m.markInitialized()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := m.poll(ctx); err != nil {
				m.logger.Warn("poll failed", slog.String("error", err.Error()))
			}
		}
	}
}

func (m *Monitor) poll(ctx context.Context) error {
	if err := m.pollAssigned(ctx); err != nil {
		return err
	}
	if err := m.pollAuthored(ctx); err != nil {
		return err
	}
	if err := cache.Save(m.cfg.CacheFile, m.state); err != nil {
		return fmt.Errorf("save cache: %w", err)
	}
	return nil
}

func (m *Monitor) pollAssigned(ctx context.Context) error {
	results, err := m.client.SearchAssignedPullRequests(ctx, m.cfg.AssignedQuery, m.cfg.MaxResults)
	if err != nil {
		return fmt.Errorf("search assigned PRs: %w", err)
	}
	for _, item := range results {
		repo, err := githubapi.RepoFromURL(item.URL)
		if err != nil {
			m.logger.Warn("failed to resolve repo from URL", slog.String("url", item.URL), slog.String("error", err.Error()))
			continue
		}
		key := prKey(repo, item.Number)

		m.mu.Lock()
		last := m.state.AssignedPRs[key]
		m.mu.Unlock()

		if !m.state.Initialized || !item.UpdatedAt.After(last) {
			if item.UpdatedAt.After(last) || last.IsZero() {
				m.mu.Lock()
				m.state.AssignedPRs[key] = item.UpdatedAt
				m.mu.Unlock()
			}
			continue
		}

		details, err := m.client.PullRequestDetails(ctx, repo, item.Number)
		if err != nil {
			m.logger.Warn("failed to load PR details", slog.String("repo", repo), slog.Int("number", item.Number), slog.String("error", err.Error()))
			continue
		}

		message := fmt.Sprintf("#%d · +%d −%d · %d files", details.Number, details.Additions, details.Deletions, details.ChangedFiles)
		if err := m.notifier.Notify(ctx, details.Title, repo, message); err != nil {
			m.logger.Warn("notification failed", slog.String("repo", repo), slog.Int("number", item.Number), slog.String("error", err.Error()))
		}

		m.mu.Lock()
		m.state.AssignedPRs[key] = item.UpdatedAt
		m.mu.Unlock()
	}
	return nil
}

func (m *Monitor) pollAuthored(ctx context.Context) error {
	results, err := m.client.ListAuthoredPullRequests(ctx, m.cfg.Author, m.cfg.MaxResults)
	if err != nil {
		return fmt.Errorf("list authored PRs: %w", err)
	}
	for _, item := range results {
		repo, err := githubapi.RepoFromURL(item.URL)
		if err != nil {
			m.logger.Warn("failed to resolve repo from URL", slog.String("url", item.URL), slog.String("error", err.Error()))
			continue
		}
		key := prKey(repo, item.Number)

		m.mu.Lock()
		record := m.state.AuthoredPRs[key]
		m.mu.Unlock()

		maxCommentTime := record.LastIssueComment
		comments, err := m.client.IssueCommentsSince(ctx, repo, item.Number, record.LastIssueComment)
		if err != nil {
			m.logger.Warn("issue comments fetch failed", slog.String("repo", repo), slog.Int("number", item.Number), slog.String("error", err.Error()))
		} else {
			for _, cmt := range comments {
				if cmt.UpdatedAt.After(maxCommentTime) {
					maxCommentTime = cmt.UpdatedAt
				}
				if !m.state.Initialized || !cmt.UpdatedAt.After(record.LastIssueComment) {
					continue
				}
				body := summarizeText(cmt.Body, 220)
				message := fmt.Sprintf("%s: %s", cmt.User.Login, body)
				subtitle := fmt.Sprintf("%s · #%d", repo, item.Number)
				if err := m.notifier.Notify(ctx, item.Title, subtitle, message); err != nil {
					m.logger.Warn("notification failed", slog.String("repo", repo), slog.Int("number", item.Number), slog.String("error", err.Error()))
				}
			}
		}

		maxReviewTime := record.LastReview
		reviews, err := m.client.Reviews(ctx, repo, item.Number)
		if err != nil {
			m.logger.Warn("pull request reviews fetch failed", slog.String("repo", repo), slog.Int("number", item.Number), slog.String("error", err.Error()))
		} else {
			for _, rvw := range reviews {
				if rvw.SubmittedAt.After(maxReviewTime) {
					maxReviewTime = rvw.SubmittedAt
				}
				if rvw.SubmittedAt.IsZero() || !m.state.Initialized || !rvw.SubmittedAt.After(record.LastReview) {
					continue
				}
				state := titleCase(rvw.State)
				body := summarizeText(rvw.Body, 180)
				if body == "" {
					body = state
				} else {
					body = fmt.Sprintf("%s — %s", state, body)
				}
				message := fmt.Sprintf("%s: %s", rvw.User.Login, body)
				subtitle := fmt.Sprintf("%s · #%d", repo, item.Number)
				if err := m.notifier.Notify(ctx, item.Title, subtitle, message); err != nil {
					m.logger.Warn("notification failed", slog.String("repo", repo), slog.Int("number", item.Number), slog.String("error", err.Error()))
				}
			}
		}

		record.LastIssueComment = maxCommentTime
		record.LastReview = maxReviewTime

		m.mu.Lock()
		m.state.AuthoredPRs[key] = record
		m.mu.Unlock()
	}
	return nil
}

func (m *Monitor) markInitialized() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Initialized {
		return
	}
	m.state.Initialized = true
}

func prKey(repo string, number int) string {
	return fmt.Sprintf("%s#%d", repo, number)
}

func summarizeText(body string, limit int) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	body = strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(body, "\n")
	trimmed := strings.TrimSpace(lines[0])
	if len(trimmed) == 0 && len(lines) > 1 {
		trimmed = strings.TrimSpace(lines[1])
	}
	runes := []rune(trimmed)
	if len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return trimmed
}

func titleCase(input string) string {
	if input == "" {
		return ""
	}
	lower := strings.ToLower(input)
	runes := []rune(lower)
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}
