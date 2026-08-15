//go:build liveapi

// Live checks against real GitLab and Gitea servers.
//
// The Cloud preview for those forges is a set of assumptions about JSON
// field names and endpoint paths — the kind of thing that compiles, passes
// every unit test, and is wrong. These tests read public repositories with
// no token, so they need no account, and they assert only what the app
// actually depends on.
//
//	go test -tags liveapi -run Live -v ./...
//
// Kept behind a build tag because they reach the network and depend on
// servers nobody here controls: a red result may mean the API changed, or
// merely that codeberg.org is down.
package main

import (
	"testing"
)

var (
	gitlabAccount = Account{Scheme: "gitlab", Host: "gitlab.com", Login: "live-test"}
	giteaAccount  = Account{Scheme: "gitea", Host: "codeberg.org", Login: "live-test"}
)

func TestLiveGitLabOverview(t *testing.T) {
	got, err := forgeRepoOverview(gitlabAccount, "", "gitlab-org/gitlab-runner")
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	assertOverview(t, got, "gitlab")
}

func TestLiveGiteaOverview(t *testing.T) {
	got, err := forgeRepoOverview(giteaAccount, "", "forgejo/forgejo")
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	assertOverview(t, got, "gitea")
}

// assertOverview checks every field the Cloud panel renders. Pipelines are
// the one exception: a repository may legitimately have none, and Gitea
// Actions is optional, so an empty CI strip is not a failure.
func assertOverview(t *testing.T, got RepoOverview, scheme string) {
	t.Helper()
	if got.Description == "" {
		t.Error("description is empty — the field name is probably wrong")
	}
	if got.DefaultBranch == "" {
		t.Fatal("default branch is empty — nothing that needs a ref can work")
	}
	if len(got.Languages) == 0 {
		t.Error("no languages — the shares are computed from the wrong shape")
	}
	for _, l := range got.Languages {
		if l.Name == "" || l.Pct <= 0 || l.Pct > 100 {
			t.Errorf("implausible language share: %+v", l)
		}
	}
	if len(got.Branches) == 0 {
		t.Error("no branches — the listing endpoint or its field changed")
	}
	if len(got.Docs) == 0 {
		t.Error("no markdown found — the tree is not being read")
	} else if got.Docs[0] != "README.md" && got.Docs[0] != "readme.md" {
		t.Errorf("README should sort first, got %q", got.Docs[0])
	}
	for _, r := range got.Runs {
		if r.Status == "" && r.Conclusion == "" {
			t.Errorf("a CI run with neither status nor conclusion: %+v", r)
		}
	}
	t.Logf("%s: branch=%s langs=%d branches=%d docs=%d runs=%d",
		scheme, got.DefaultBranch, len(got.Languages), len(got.Branches), len(got.Docs), len(got.Runs))
}

func TestLiveGitLabDoc(t *testing.T) {
	assertReadable(t, gitlabAccount, "gitlab-org/gitlab-runner")
}

func TestLiveGiteaDoc(t *testing.T) {
	assertReadable(t, giteaAccount, "forgejo/forgejo")
}

// assertReadable reads the README the overview offered, which is the exact
// path the doc reader is given when a user opens a repository.
func assertReadable(t *testing.T, acc Account, fullName string) {
	t.Helper()
	overview, err := forgeRepoOverview(acc, "", fullName)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if len(overview.Docs) == 0 {
		t.Fatal("no docs to read")
	}
	text, err := forgeRepoDoc(acc, "", fullName, overview.Docs[0])
	if err != nil {
		t.Fatalf("reading %s: %v", overview.Docs[0], err)
	}
	if len(text) < 100 {
		t.Errorf("%s came back as %d bytes — probably still base64 or an error body",
			overview.Docs[0], len(text))
	}
	t.Logf("%s: %s is %d bytes", acc.Scheme, overview.Docs[0], len(text))
}

func TestLiveTemplateDetection(t *testing.T) {
	cases := []struct {
		name     string
		acc      Account
		fullName string
		branch   string
		want     bool
	}{
		// Neither is a Templetry template, so the honest assertion is that
		// the probe completes and says no rather than erroring into a false
		// negative that would look identical.
		{"gitlab", gitlabAccount, "gitlab-org/gitlab-runner", "main", false},
		{"gitea", giteaAccount, "forgejo/forgejo", "forgejo", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := forgeHasTemplate(c.acc, "", c.fullName, c.branch); got != c.want {
				t.Errorf("forgeHasTemplate = %v, want %v", got, c.want)
			}
		})
	}
}
