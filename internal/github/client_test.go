package github

import "testing"

func TestRepoFromURL(t *testing.T) {
	repo, err := RepoFromURL("https://github.com/deseretdigital/example/pull/123")
	if err != nil {
		t.Fatalf("RepoFromURL error = %v", err)
	}
	if repo != "deseretdigital/example" {
		t.Fatalf("unexpected repo = %q", repo)
	}
}

func TestRepoFromURLErrorsOnShortPath(t *testing.T) {
	if _, err := RepoFromURL("https://github.com/deseretdigital"); err == nil {
		t.Fatalf("expected error for short path")
	}
}
