package main

import (
	"testing"

	"github.com/Templetry/engine/catalog"
)

// pieceRegistry merges the common pieces of every loaded catalog. The
// official one has to come first, because piece.Resolve walks the list in
// order and keeps the first entry that supports the project's template —
// so ordering decides which implementation a user gets.
func TestPieceRegistryPutsOfficialFirst(t *testing.T) {
	a := &App{regs: map[string]*catalog.Registry{
		"zzz-custom": {Pieces: []catalog.CommonPiece{
			{Name: "audit-trail", Repo: "acme/pieces", Path: "audit"},
		}},
		OfficialCatalogName: {Pieces: []catalog.CommonPiece{
			{Name: "renovate", Repo: "Templetry/pieces", Path: "renovate"},
		}},
		"aaa-custom": {Pieces: []catalog.CommonPiece{
			{Name: "outbox", Repo: "acme/pieces", Path: "outbox"},
		}},
	}}

	got := a.pieceRegistry().Pieces
	if len(got) != 3 {
		t.Fatalf("want 3 pieces, got %d", len(got))
	}
	if got[0].Name != "renovate" {
		t.Errorf("official catalog must come first, got %q", got[0].Name)
	}
	// The rest are sorted by catalog name, so the merge is deterministic
	// rather than dependent on map iteration order.
	if got[1].Name != "outbox" || got[2].Name != "audit-trail" {
		t.Errorf("custom catalogs must merge in name order, got %q then %q", got[1].Name, got[2].Name)
	}
}

// The same piece name may appear once per ecosystem, so identity is the
// name plus where it comes from — deduping by name alone would hide
// implementations and break ADR-0016's "one name, many implementations".
func TestPieceRegistryKeepsSameNameFromDifferentSources(t *testing.T) {
	a := &App{regs: map[string]*catalog.Registry{
		OfficialCatalogName: {Pieces: []catalog.CommonPiece{
			{Name: "audit-trail", Repo: "Templetry/pieces", Path: "audit-trail-go-sqlite"},
			{Name: "audit-trail", Repo: "Templetry/pieces", Path: "audit-trail-python"},
			{Name: "audit-trail", Repo: "Templetry/pieces", Path: "audit-trail-go-sqlite"}, // exact dupe
		}},
	}}

	got := a.pieceRegistry().Pieces
	if len(got) != 2 {
		t.Fatalf("want the two distinct implementations, got %d", len(got))
	}
	if got[0].Path != "audit-trail-go-sqlite" || got[1].Path != "audit-trail-python" {
		t.Errorf("declared order must survive, got %q then %q", got[0].Path, got[1].Path)
	}
}

// Gitea signs git pushes in with plain HTTP basic and wants the real
// username; GitHub and GitLab both accept "oauth2" with the token as
// password. Getting this wrong makes pushes fail with no useful message.
func TestGitAuthForUsesTheRightCredentialUser(t *testing.T) {
	cases := []struct {
		scheme, host, login string
		wantUser, wantMail  string
	}{
		{"github", "github.com", "sebss", "oauth2", "sebss@users.noreply.github.com"},
		{"gitlab", "gitlab.com", "sebss", "oauth2", "sebss@users.noreply.gitlab.com"},
		{"gitea", "codeberg.org", "sebss", "sebss", "sebss@users.noreply.codeberg.org"},
	}
	for _, c := range cases {
		got := gitAuthFor(Account{Scheme: c.scheme, Host: c.host, Login: c.login}, "tok")
		if got.User != c.wantUser {
			t.Errorf("%s: credential user = %q, want %q", c.scheme, got.User, c.wantUser)
		}
		if got.Email != c.wantMail {
			t.Errorf("%s: email = %q, want %q", c.scheme, got.Email, c.wantMail)
		}
	}
}

// The credential username is interpolated into a shell snippet, so it must
// not be able to carry anything but a name.
func TestSafeGitUserStripsShellMetacharacters(t *testing.T) {
	cases := map[string]string{
		`ada`:                  "ada",
		`ada.lovelace-1_x`:     "ada.lovelace-1_x",
		`ada"; rm -rf /; echo`: "adarm-rfecho",
		`$(whoami)`:            "whoami",
		``:                     "oauth2",
		`;;;`:                  "oauth2",
	}
	for in, want := range cases {
		if got := safeGitUser(in); got != want {
			t.Errorf("safeGitUser(%q) = %q, want %q", in, got, want)
		}
	}
}

// repoKey has to separate forges: the same owner/name really can exist on
// GitHub and on a company GitLab, and flagging one would flag both.
func TestRepoKeySeparatesForges(t *testing.T) {
	if repoKey("", "Acme/App") == repoKey("gitlab@gitlab.acme.com", "Acme/App") {
		t.Error("same owner/name on two forges must not share a key")
	}
	if repoKey("", "Acme/App") != repoKey("", "acme/app") {
		t.Error("keys must be case-insensitive")
	}
}
