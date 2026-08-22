package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseCredential(t *testing.T) {
	// Every account added before OAuth existed holds a bare token. Reading
	// one has to keep working, or signing in again becomes the upgrade path.
	t.Run("a bare token is a personal access token", func(t *testing.T) {
		c := parseCredential("glpat-abc123")
		if c.Access != "glpat-abc123" || c.Refresh != "" || c.Expires != 0 {
			t.Fatalf("got %+v", c)
		}
	})

	t.Run("surrounding whitespace is not part of the token", func(t *testing.T) {
		if got := parseCredential("  glpat-abc123\n").Access; got != "glpat-abc123" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("a grant round-trips", func(t *testing.T) {
		want := storedCredential{Access: "at", Refresh: "rt", Expires: 1770000000}
		got := parseCredential(want.encode())
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	// Handing a forge a blob of braces produces an authentication error that
	// says nothing about what is actually wrong.
	t.Run("malformed JSON is not treated as a token", func(t *testing.T) {
		if got := parseCredential(`{"access":`).Access; got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestCredentialEncodeKeepsPATsPlain(t *testing.T) {
	// A PAT that came back out as JSON would break every older build of the
	// app reading the same keyring entry.
	if got := (storedCredential{Access: "glpat-x"}).encode(); got != "glpat-x" {
		t.Errorf("got %q", got)
	}
}

func TestCredentialExpiry(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	cases := []struct {
		name string
		c    storedCredential
		want bool
	}{
		{"a PAT never expires", storedCredential{Access: "p"}, false},
		// Nothing could be done about it, and saying so would turn a working
		// token into an error message.
		{"no refresh token means nothing to renew with",
			storedCredential{Access: "a", Expires: now.Add(-time.Hour).Unix()}, false},
		{"a fresh grant is fine",
			storedCredential{Access: "a", Refresh: "r", Expires: now.Add(time.Hour).Unix()}, false},
		{"an expired grant needs renewing",
			storedCredential{Access: "a", Refresh: "r", Expires: now.Add(-time.Second).Unix()}, true},
		// A clone can outlive the seconds left on a token, and a refresh
		// halfway through is a failure the user cannot act on.
		{"a grant expiring within the margin is renewed early",
			storedCredential{Access: "a", Refresh: "r", Expires: now.Add(30 * time.Second).Unix()}, true},
		{"just outside the margin is left alone",
			storedCredential{Access: "a", Refresh: "r", Expires: now.Add(3 * time.Minute).Unix()}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.c.expired(now); got != c.want {
				t.Errorf("expired = %v, want %v", got, c.want)
			}
		})
	}
}

func TestTokensToCredential(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	got := oauthTokens{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 7200}.credential(now)
	if got.Expires != now.Add(2*time.Hour).Unix() {
		t.Errorf("expires = %d", got.Expires)
	}

	// GitHub's OAuth tokens carry no lifetime, and inventing one would make
	// the app renew something that never needed renewing.
	noExpiry := oauthTokens{AccessToken: "at"}.credential(now)
	if noExpiry.Expires != 0 {
		t.Errorf("expires = %d, want 0", noExpiry.Expires)
	}
}

func TestEndpoints(t *testing.T) {
	if got := tokenEndpoint("github", "github.com"); !strings.Contains(got, "github.com/login/oauth") {
		t.Errorf("github token endpoint = %q", got)
	}
	if got := tokenEndpoint("gitlab", "gitlab.example.com"); got != "https://gitlab.example.com/oauth/token" {
		t.Errorf("gitlab token endpoint = %q", got)
	}
	if got := deviceEndpoint("gitlab", "gitlab.com"); got != "https://gitlab.com/oauth/authorize_device" {
		t.Errorf("gitlab device endpoint = %q", got)
	}
}

func TestSupportsDeviceFlow(t *testing.T) {
	// Gitea implements only the authorization code grant, so no application
	// id would help.
	if supportsDeviceFlow("gitea", "codeberg.org") {
		t.Error("gitea must not claim device flow support")
	}
	// Without an application id there is nothing to present, so the app has
	// to keep asking for a token instead of offering a sign-in that fails.
	if supportsDeviceFlow("gitlab", "gitlab.example.com") {
		t.Error("an unconfigured host must not claim device flow support")
	}
	t.Setenv("TEMPLETRY_OAUTH_CLIENT_ID_GITLAB", "test-id")
	if !supportsDeviceFlow("gitlab", "gitlab.example.com") {
		t.Error("a configured host must claim support")
	}
}

func TestRefreshCredential(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	old := storedCredential{Access: "old", Refresh: "rt-old", Expires: now.Unix()}

	t.Run("exchanges the refresh token for a new pair", func(t *testing.T) {
		var got map[string]string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			got = map[string]string{
				"grant_type":    r.FormValue("grant_type"),
				"refresh_token": r.FormValue("refresh_token"),
				"client_id":     r.FormValue("client_id"),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(oauthTokens{
				AccessToken: "new", RefreshToken: "rt-new", ExpiresIn: 7200,
			})
		}))
		defer srv.Close()

		next, err := refreshAt(srv.URL, "cid", old, now)
		if err != nil {
			t.Fatal(err)
		}
		if got["grant_type"] != "refresh_token" || got["refresh_token"] != "rt-old" || got["client_id"] != "cid" {
			t.Errorf("sent %+v", got)
		}
		// GitLab replaces both on use; keeping the old refresh token would
		// leave the account dead after the next renewal.
		if next.Access != "new" || next.Refresh != "rt-new" {
			t.Errorf("got %+v", next)
		}
		if next.Expires != now.Add(2*time.Hour).Unix() {
			t.Errorf("expires = %d", next.Expires)
		}
	})

	t.Run("a refused refresh says to sign in again", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(oauthTokens{
				Error: "invalid_grant", ErrorDesc: "refresh token is invalid",
			})
		}))
		defer srv.Close()

		_, err := refreshAt(srv.URL, "cid", old, now)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "sign in again") {
			t.Errorf("error does not say what to do: %v", err)
		}
	})

	t.Run("declines without a refresh token", func(t *testing.T) {
		if _, err := refreshAt("http://unused", "cid", storedCredential{Access: "a"}, now); err == nil {
			t.Error("expected an error")
		}
	})
}
