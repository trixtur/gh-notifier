package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Client wraps the GitHub CLI for JSON-centric operations.
type Client struct {
	binary string
	logger *slog.Logger
}

func NewClient(logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	return &Client{
		binary: "gh",
		logger: logger,
	}
}

type PullRequest struct {
	Number       int       `json:"number"`
	Title        string    `json:"title"`
	URL          string    `json:"url"`
	UpdatedAt    time.Time `json:"updatedAt"`
	Additions    int       `json:"additions"`
	Deletions    int       `json:"deletions"`
	ChangedFiles int       `json:"changedFiles"`
}

type PullRequestSummary struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type IssueComment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	UpdatedAt time.Time `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	HTMLURL string `json:"html_url"`
}

type Review struct {
	ID          int64     `json:"id"`
	Body        string    `json:"body"`
	State       string    `json:"state"`
	SubmittedAt time.Time `json:"submitted_at"`
	User        struct {
		Login string `json:"login"`
	} `json:"user"`
	HTMLURL string `json:"html_url"`
}

func (c *Client) CurrentUserLogin(ctx context.Context) (string, error) {
	out, err := c.run(ctx, "api", "user", "--jq", ".login")
	if err != nil {
		return "", err
	}
	login := strings.TrimSpace(string(out))
	if login == "" {
		return "", fmt.Errorf("gh returned empty login")
	}
	return login, nil
}

func (c *Client) SearchAssignedPullRequests(ctx context.Context, query string, limit int) ([]PullRequestSummary, error) {
	args := []string{"search", "prs", "--search", query, "--sort", "updated", "--order", "desc", "--json", "number,title,url,updatedAt"}
	if limit > 0 {
		args = append(args, "--limit", strconv.Itoa(limit))
	}
	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var prs []PullRequestSummary
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("decode pull request search results: %w", err)
	}
	return prs, nil
}

func (c *Client) ListAuthoredPullRequests(ctx context.Context, author string, limit int) ([]PullRequestSummary, error) {
	args := []string{"pr", "list", "--author", author, "--state", "open", "--json", "number,title,url,updatedAt"}
	if limit > 0 {
		args = append(args, "--limit", strconv.Itoa(limit))
	}
	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var prs []PullRequestSummary
	if len(bytes.TrimSpace(out)) == 0 {
		return prs, nil
	}
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("decode authored pull request list: %w", err)
	}
	return prs, nil
}

func (c *Client) PullRequestDetails(ctx context.Context, repo string, number int) (*PullRequest, error) {
	args := []string{
		"pr", "view", strconv.Itoa(number),
		"--repo", repo,
		"--json", "number,title,url,updatedAt,additions,deletions,changedFiles",
	}
	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var pr PullRequest
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil, fmt.Errorf("decode pull request details: %w", err)
	}
	return &pr, nil
}

func (c *Client) IssueCommentsSince(ctx context.Context, repo string, number int, since time.Time) ([]IssueComment, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/comments", repo, number)
	args := []string{"api", path, "--method", "GET", "-F", "per_page=100"}
	if !since.IsZero() {
		args = append(args, "-F", "since="+since.Format(time.RFC3339))
	}
	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
	}
	var comments []IssueComment
	if err := json.Unmarshal(out, &comments); err != nil {
		return nil, fmt.Errorf("decode issue comments: %w", err)
	}
	return comments, nil
}

func (c *Client) Reviews(ctx context.Context, repo string, number int) ([]Review, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/reviews", repo, number)
	args := []string{"api", path, "--method", "GET", "-F", "per_page=100"}
	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
	}
	var reviews []Review
	if err := json.Unmarshal(out, &reviews); err != nil {
		return nil, fmt.Errorf("decode pull request reviews: %w", err)
	}
	return reviews, nil
}

func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.binary, args...)
	cmd.Env = append(os.Environ(),
		"GH_PAGER=",
		"GH_PROMPT_DISABLED=1",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), errMsg)
	}

	return stdout.Bytes(), nil
}

func RepoFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 {
		return "", fmt.Errorf("url path too short: %s", u.Path)
	}
	return fmt.Sprintf("%s/%s", parts[0], parts[1]), nil
}
