package monitor

import (
	"context"
	"testing"
	"time"

	"gh-review-notifier/internal/cache"
	githubapi "gh-review-notifier/internal/github"
)

type fakeGitHubClient struct {
	assigned []githubapi.PullRequestSummary
	authored []githubapi.PullRequestSummary

	prDetails map[string]*githubapi.PullRequest

	issueComments map[string][]githubapi.IssueComment
	reviews       map[string][]githubapi.Review
}

func (f *fakeGitHubClient) SearchAssignedPullRequests(ctx context.Context, query string, limit int) ([]githubapi.PullRequestSummary, error) {
	return f.assigned, nil
}

func (f *fakeGitHubClient) ListAuthoredPullRequests(ctx context.Context, author string, limit int) ([]githubapi.PullRequestSummary, error) {
	return f.authored, nil
}

func (f *fakeGitHubClient) PullRequestDetails(ctx context.Context, repo string, number int) (*githubapi.PullRequest, error) {
	key := prKey(repo, number)
	if pr, ok := f.prDetails[key]; ok {
		return pr, nil
	}
	return nil, nil
}

func (f *fakeGitHubClient) IssueCommentsSince(ctx context.Context, repo string, number int, since time.Time) ([]githubapi.IssueComment, error) {
	key := prKey(repo, number)
	return f.issueComments[key], nil
}

func (f *fakeGitHubClient) Reviews(ctx context.Context, repo string, number int) ([]githubapi.Review, error) {
	key := prKey(repo, number)
	return f.reviews[key], nil
}

type notification struct {
	title    string
	subtitle string
	message  string
}

type fakeNotifier struct {
	notifications []notification
}

func (f *fakeNotifier) Notify(ctx context.Context, title, subtitle, message string) error {
	f.notifications = append(f.notifications, notification{
		title:    title,
		subtitle: subtitle,
		message:  message,
	})
	return nil
}

func TestPollAssignedNewActivity(t *testing.T) {
	ctx := context.Background()
	state := cache.NewState()
	state.Initialized = true
	state.AssignedPRs["deseretdigital/example#42"] = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	updatedTime := time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC)

	client := &fakeGitHubClient{
		assigned: []githubapi.PullRequestSummary{
			{
				Number:    42,
				Title:     "Improve observability",
				URL:       "https://github.com/deseretdigital/example/pull/42",
				UpdatedAt: updatedTime,
			},
		},
		prDetails: map[string]*githubapi.PullRequest{
			"deseretdigital/example#42": {
				Number:       42,
				Title:        "Improve observability",
				Additions:    120,
				Deletions:    30,
				ChangedFiles: 5,
			},
		},
		issueComments: make(map[string][]githubapi.IssueComment),
		reviews:       make(map[string][]githubapi.Review),
	}

	notifier := &fakeNotifier{}
	mon := NewMonitor(Config{AssignedQuery: "ignored"}, client, notifier, state, nil)

	if err := mon.pollAssigned(ctx); err != nil {
		t.Fatalf("pollAssigned error = %v", err)
	}

	if len(notifier.notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.notifications))
	}

	got := notifier.notifications[0]
	if got.title != "Improve observability" {
		t.Errorf("notification title = %q", got.title)
	}
	if got.subtitle != "deseretdigital/example" {
		t.Errorf("notification subtitle = %q", got.subtitle)
	}
	expectedMsg := "#42 · +120 −30 · 5 files"
	if got.message != expectedMsg {
		t.Errorf("notification message = %q, want %q", got.message, expectedMsg)
	}

	if state.AssignedPRs["deseretdigital/example#42"] != updatedTime {
		t.Errorf("state not updated with latest timestamp")
	}
}

func TestPollAssignedInitialSyncSuppressesNotifications(t *testing.T) {
	ctx := context.Background()
	state := cache.NewState()
	state.Initialized = false

	updatedTime := time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC)

	client := &fakeGitHubClient{
		assigned: []githubapi.PullRequestSummary{
			{
				Number:    43,
				Title:     "Add metrics endpoint",
				URL:       "https://github.com/deseretdigital/example/pull/43",
				UpdatedAt: updatedTime,
			},
		},
		prDetails:     make(map[string]*githubapi.PullRequest),
		issueComments: make(map[string][]githubapi.IssueComment),
		reviews:       make(map[string][]githubapi.Review),
	}

	notifier := &fakeNotifier{}
	mon := NewMonitor(Config{}, client, notifier, state, nil)

	if err := mon.pollAssigned(ctx); err != nil {
		t.Fatalf("pollAssigned error = %v", err)
	}

	if len(notifier.notifications) != 0 {
		t.Fatalf("expected 0 notifications, got %d", len(notifier.notifications))
	}

	if ts := state.AssignedPRs["deseretdigital/example#43"]; !ts.Equal(updatedTime) {
		t.Fatalf("expected state timestamp=%v, got %v", updatedTime, ts)
	}
}

func TestPollAuthoredNotifiesOnNewCommentAndReview(t *testing.T) {
	ctx := context.Background()
	state := cache.NewState()
	state.Initialized = true
	state.AuthoredPRs["deseretdigital/example#99"] = cache.AuthoredRecord{
		LastIssueComment: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		LastReview:       time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC),
	}

	commentTime := time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC)
	reviewTime := time.Date(2024, 1, 1, 13, 30, 0, 0, time.UTC)

	client := &fakeGitHubClient{
		authored: []githubapi.PullRequestSummary{
			{
				Number:    99,
				Title:     "Refactor data pipeline",
				URL:       "https://github.com/deseretdigital/example/pull/99",
				UpdatedAt: reviewTime,
			},
		},
		issueComments: map[string][]githubapi.IssueComment{
			"deseretdigital/example#99": {
				{
					ID:        1,
					Body:      "Looks good overall!",
					UpdatedAt: commentTime,
					User: struct {
						Login string `json:"login"`
					}{Login: "teammate"},
				},
			},
		},
		reviews: map[string][]githubapi.Review{
			"deseretdigital/example#99": {
				{
					ID:          2,
					Body:        "Approved with minor nits.",
					State:       "approved",
					SubmittedAt: reviewTime,
					User: struct {
						Login string `json:"login"`
					}{Login: "lead"},
				},
			},
		},
		prDetails: make(map[string]*githubapi.PullRequest),
	}

	notifier := &fakeNotifier{}
	mon := NewMonitor(Config{Author: "trixtur"}, client, notifier, state, nil)

	if err := mon.pollAuthored(ctx); err != nil {
		t.Fatalf("pollAuthored error = %v", err)
	}

	if len(notifier.notifications) != 2 {
		t.Fatalf("expected 2 notifications (comment + review), got %d", len(notifier.notifications))
	}

	commentNotif := notifier.notifications[0]
	if commentNotif.message != "teammate: Looks good overall!" {
		t.Errorf("comment notification message = %q", commentNotif.message)
	}

	reviewNotif := notifier.notifications[1]
	if reviewNotif.message != "lead: Approved — Approved with minor nits." {
		t.Errorf("review notification message = %q", reviewNotif.message)
	}

	record := state.AuthoredPRs["deseretdigital/example#99"]
	if !record.LastIssueComment.Equal(commentTime) {
		t.Errorf("LastIssueComment not updated: %v", record.LastIssueComment)
	}
	if !record.LastReview.Equal(reviewTime) {
		t.Errorf("LastReview not updated: %v", record.LastReview)
	}
}
