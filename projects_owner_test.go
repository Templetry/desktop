package main

import "testing"

func TestOwnerFromRemote(t *testing.T) {
	cases := []struct{ remote, host, owner string }{
		{"https://github.com/Templetry/engine.git", "github.com", "Templetry"},
		{"https://github.com/Templetry/engine", "github.com", "Templetry"},
		{"git@github.com:Templetry/engine.git", "github.com", "Templetry"},
		{"ssh://git@codeberg.org/user/repo.git", "codeberg.org", "user"},
		// A GitLab project can sit several groups deep, and each level is
		// part of who owns it.
		{"https://gitlab.com/group/sub/proj.git", "gitlab.com", "group/sub"},
		{"git@gitlab.com:group/sub/proj.git", "gitlab.com", "group/sub"},
		// Credentials in the URL must not be mistaken for the host.
		{"https://token@github.com/Owner/repo.git", "github.com", "Owner"},
		// Nothing to derive: say so rather than guessing.
		{"", "", ""},
		{"/home/me/local-only", "", ""},
		{"https://github.com/orphan", "github.com", ""},
	}
	for _, c := range cases {
		host, owner := ownerFromRemote(c.remote)
		if host != c.host || owner != c.owner {
			t.Errorf("ownerFromRemote(%q) = (%q, %q), want (%q, %q)", c.remote, host, owner, c.host, c.owner)
		}
	}
}

func TestAvatarFor(t *testing.T) {
	if got := avatarFor("github.com", "Templetry"); got != "https://github.com/Templetry.png" {
		t.Errorf("got %q", got)
	}
	// Nested groups hang off one account, and that is whose avatar it is.
	if got := avatarFor("gitlab.com", "group/sub"); got != "https://gitlab.com/group.png" {
		t.Errorf("got %q", got)
	}
	// No owner, no guess — the UI shows a placeholder instead.
	if got := avatarFor("github.com", ""); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
