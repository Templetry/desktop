package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

// What a forge account keeps in the OS keyring.
//
// A personal access token is stored as itself — a bare string — which is what
// every account added before this existed contains, and what a PAT will always
// be: it does not expire on a schedule and there is nothing else to remember.
//
// An OAuth grant needs more than a string. GitLab issues access tokens that
// expire in two hours and a refresh token that replaces both when used, so the
// three facts travel together as JSON. Reading tolerates either shape, which is
// what lets an existing installation keep working without a migration step.
type storedCredential struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh,omitempty"`
	// Unix seconds. Zero means "does not expire", which is true of a PAT and
	// of a GitHub OAuth token.
	Expires int64 `json:"expires,omitempty"`
}

// refreshMargin is how early a token is replaced. A clone or a push can take
// longer than the seconds left on a token, and a refresh halfway through is a
// failure the user cannot act on — so the check happens before the work starts,
// with room to spare.
const refreshMargin = 2 * time.Minute

func parseCredential(raw string) storedCredential {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "{") {
		var c storedCredential
		if err := json.Unmarshal([]byte(s), &c); err == nil && c.Access != "" {
			return c
		}
		// Malformed JSON is not a token. Falling through would hand the
		// forge a blob of braces and produce an authentication error that
		// says nothing about what is wrong.
		return storedCredential{}
	}
	return storedCredential{Access: s}
}

func (c storedCredential) encode() string {
	if c.Refresh == "" && c.Expires == 0 {
		return c.Access // a PAT stays a PAT
	}
	b, err := json.Marshal(c)
	if err != nil {
		return c.Access
	}
	return string(b)
}

// expired reports whether the credential should be replaced before use.
// A credential with no refresh token is never "expired" here: there would be
// nothing to do about it, and saying so would only turn a working token into
// an error message.
func (c storedCredential) expired(now time.Time) bool {
	return c.Refresh != "" && c.Expires > 0 && now.Add(refreshMargin).Unix() >= c.Expires
}

// oauthTokens is the shape every OAuth token endpoint answers with.
type oauthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func (t oauthTokens) credential(now time.Time) storedCredential {
	c := storedCredential{Access: t.AccessToken, Refresh: t.RefreshToken}
	if t.ExpiresIn > 0 {
		c.Expires = now.Add(time.Duration(t.ExpiresIn) * time.Second).Unix()
	}
	return c
}

// gitlabClientID is the public OAuth application id for gitlab.com, the one
// instance Templetry can ship a client for. Every other GitLab is somebody
// else's server with its own applications, which is why a host can carry its
// own id in settings.
//
// The application is public and non-confidential: the device flow presents
// this id and no secret, which is what makes it safe to ship. The token it
// yields belongs to whoever authorizes it, never to this id.
const gitlabClientID = "d03abd86d938ddf9d9eca771cad283ae070055748b7047855279a17a1f256f1e"

// oauthClientID returns the application id to use for a host: the one shipped
// for a forge Templetry has registered with, or the one the user configured
// for their own server.
func oauthClientID(scheme, host string) string {
	if v := os.Getenv("TEMPLETRY_OAUTH_CLIENT_ID_" + strings.ToUpper(scheme)); v != "" {
		return v
	}
	if id := loadConfig().OAuthClients[host]; id != "" {
		return id
	}
	switch {
	case scheme == "github":
		return clientID()
	case host == "gitlab.com":
		return gitlabClientID
	}
	return ""
}

// supportsDeviceFlow reports whether signing in without a token is possible
// for a host. Gitea implements only the authorization code grant, so it never
// is; anywhere else it depends on having an application id to present.
func supportsDeviceFlow(scheme, host string) bool {
	if scheme == "gitea" {
		return false
	}
	return oauthClientID(scheme, host) != ""
}

// tokenEndpoint is where a forge exchanges and refreshes tokens.
func tokenEndpoint(scheme, host string) string {
	if scheme == "github" {
		return "https://github.com/login/oauth/access_token"
	}
	return "https://" + host + "/oauth/token"
}

// deviceEndpoint is where a device flow starts.
func deviceEndpoint(scheme, host string) string {
	if scheme == "github" {
		return "https://github.com/login/device/code"
	}
	// GitLab 17.1+ and Forgejo. Gitea implements only the authorization
	// code grant, which is why it stays on a personal access token.
	return "https://" + host + "/oauth/authorize_device"
}

// refreshCredential trades a refresh token for a new pair. GitLab invalidates
// both on use, so the result has to be stored even when the caller then fails
// for an unrelated reason — otherwise the old refresh token is gone and the
// account is dead until someone signs in again.
func refreshCredential(scheme, host, clientID string, c storedCredential, now time.Time) (storedCredential, error) {
	if clientID == "" {
		return c, fmt.Errorf("no OAuth client id configured for %s", host)
	}
	next, err := refreshAt(tokenEndpoint(scheme, host), clientID, c, now)
	if err != nil {
		return c, fmt.Errorf("%s: %w", host, err)
	}
	return next, nil
}

// refreshAt is the exchange itself, with the endpoint passed in so it can be
// pointed at a test server.
func refreshAt(endpoint, clientID string, c storedCredential, now time.Time) (storedCredential, error) {
	if c.Refresh == "" {
		return c, fmt.Errorf("no refresh token to renew with")
	}
	res, err := postForm(endpoint, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {c.Refresh},
		"client_id":     {clientID},
	})
	if err != nil {
		return c, fmt.Errorf("renewing the session: %w", err)
	}
	var t oauthTokens
	b, _ := json.Marshal(res)
	_ = json.Unmarshal(b, &t)
	if t.Error != "" || t.AccessToken == "" {
		msg := t.ErrorDesc
		if msg == "" {
			msg = t.Error
		}
		if msg == "" {
			msg = "no access token in the response"
		}
		return c, fmt.Errorf("the session could not be renewed (%s) — sign in again", msg)
	}
	return t.credential(now), nil
}

// accountCredential returns a usable access token for an account, renewing it
// first when it is about to expire. Every path that talks to a forge goes
// through here, so a two-hour token is invisible to the rest of the app.
func accountCredential(acc Account) (string, error) {
	raw, err := keyring.Get(keyringService, acc.Key())
	if err != nil {
		return "", err
	}
	c := parseCredential(raw)
	if c.Access == "" {
		return "", fmt.Errorf("no usable token stored for %s — sign in again", acc.Key())
	}
	if !c.expired(time.Now()) {
		return c.Access, nil
	}
	next, err := refreshCredential(acc.Scheme, acc.Host, oauthClientID(acc.Scheme, acc.Host), c, time.Now())
	if err != nil {
		return "", err
	}
	// Store before returning: the old refresh token is already spent.
	if err := keyring.Set(keyringService, acc.Key(), next.encode()); err != nil {
		return "", fmt.Errorf("renewed the %s session but could not store it: %w", acc.Host, err)
	}
	return next.Access, nil
}
