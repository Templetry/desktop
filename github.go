package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zalando/go-keyring"
)

// githubClientID is the public OAuth App client id (device flow enabled).
// Overridable via TEMPLETRY_GH_CLIENT_ID for development.
const githubClientID = "Ov23lisEA5jd9kGIHJLW"

const (
	keyringService = "Templetry"
	keyringUser    = "github-token"
)

func clientID() string {
	if v := os.Getenv("TEMPLETRY_GH_CLIENT_ID"); v != "" {
		return v
	}
	return githubClientID
}

// AuthStatus is what the UI renders in the auth corner.
type AuthStatus struct {
	State           string `json:"state"` // logged_out | pending | logged_in | error
	Login           string `json:"login,omitempty"`
	Avatar          string `json:"avatar,omitempty"`
	UserCode        string `json:"userCode,omitempty"`
	VerificationURI string `json:"verificationUri,omitempty"`
	Error           string `json:"error,omitempty"`
}

func (a *App) setAuth(s AuthStatus) {
	a.mu.Lock()
	a.auth = s
	a.mu.Unlock()
}

// GetAuthStatus reports the current auth state, restoring a stored token on
// first call.
func (a *App) GetAuthStatus() AuthStatus {
	a.mu.Lock()
	current := a.auth
	restored := a.authRestored
	a.authRestored = true
	a.mu.Unlock()
	if current.State != "" || restored {
		return current
	}
	token, err := keyring.Get(keyringService, keyringUser)
	if err != nil || token == "" {
		s := AuthStatus{State: "logged_out"}
		a.setAuth(s)
		return s
	}
	login, avatar, err := a.fetchLogin(token)
	if err != nil {
		_ = keyring.Delete(keyringService, keyringUser)
		s := AuthStatus{State: "logged_out"}
		a.setAuth(s)
		return s
	}
	a.mu.Lock()
	a.token = token
	a.mu.Unlock()
	s := AuthStatus{State: "logged_in", Login: login, Avatar: avatar}
	a.setAuth(s)
	return s
}

func (a *App) fetchLogin(token string) (string, string, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub /user: HTTP %d", resp.StatusCode)
	}
	var u struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return "", "", err
	}
	return u.Login, u.AvatarURL, nil
}

func postForm(u string, form url.Values) (map[string]any, error) {
	req, _ := http.NewRequest("POST", u, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// StartGitHubLogin begins the OAuth device flow: returns the user code,
// opens the verification page, and polls for the token in the background.
func (a *App) StartGitHubLogin() (AuthStatus, error) {
	id := clientID()
	if id == "" {
		return AuthStatus{}, fmt.Errorf("no GitHub OAuth App configured: set the client id first")
	}
	res, err := postForm("https://github.com/login/device/code", url.Values{
		"client_id": {id}, "scope": {"repo workflow"},
	})
	if err != nil {
		return AuthStatus{}, fmt.Errorf("starting device flow: %w", err)
	}
	deviceCode, _ := res["device_code"].(string)
	userCode, _ := res["user_code"].(string)
	verURI, _ := res["verification_uri"].(string)
	interval, _ := res["interval"].(float64)
	if deviceCode == "" || userCode == "" {
		return AuthStatus{}, fmt.Errorf("device flow rejected: %v", res["error_description"])
	}
	if interval < 5 {
		interval = 5
	}
	s := AuthStatus{State: "pending", UserCode: userCode, VerificationURI: verURI}
	a.setAuth(s)
	runtime.BrowserOpenURL(a.ctx, verURI)

	go func() {
		for {
			time.Sleep(time.Duration(interval) * time.Second)
			res, err := postForm("https://github.com/login/oauth/access_token", url.Values{
				"client_id":   {id},
				"device_code": {deviceCode},
				"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			})
			if err != nil {
				continue
			}
			if token, _ := res["access_token"].(string); token != "" {
				login, avatar, err := a.fetchLogin(token)
				if err != nil {
					a.setAuth(AuthStatus{State: "error", Error: err.Error()})
					return
				}
				_ = keyring.Set(keyringService, keyringUser, token)
				a.mu.Lock()
				a.token = token
				a.mu.Unlock()
				a.setAuth(AuthStatus{State: "logged_in", Login: login, Avatar: avatar})
				return
			}
			switch res["error"] {
			case "authorization_pending":
				continue
			case "slow_down":
				interval += 5
			case "expired_token", "access_denied":
				a.setAuth(AuthStatus{State: "logged_out", Error: fmt.Sprint(res["error_description"])})
				return
			}
		}
	}()
	return s, nil
}

// Logout forgets the stored token.
func (a *App) Logout() {
	_ = keyring.Delete(keyringService, keyringUser)
	a.mu.Lock()
	a.token = ""
	a.mu.Unlock()
	a.setAuth(AuthStatus{State: "logged_out"})
}

// PublishProject creates a GitHub repo for a rendered project and pushes it.
func (a *App) PublishProject(dir, name, description string, private bool) (string, error) {
	a.mu.Lock()
	token := a.token
	login := a.auth.Login
	a.mu.Unlock()
	if token == "" {
		return "", fmt.Errorf("sign in with GitHub first")
	}
	if dir == "" || name == "" {
		return "", fmt.Errorf("project directory and repository name are required")
	}

	body, _ := json.Marshal(map[string]any{
		"name": name, "description": description, "private": private,
	})
	req, _ := http.NewRequest("POST", "https://api.github.com/user/repos", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var repo struct {
		HTMLURL  string `json:"html_url"`
		CloneURL string `json:"clone_url"`
		Message  string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("creating repository: %s (HTTP %d)", repo.Message, resp.StatusCode)
	}

	gh := Account{Scheme: "github", Host: "github.com", Login: login}
	if err := gitPublish(dir, repo.CloneURL, gitAuthFor(gh, token)); err != nil {
		return "", fmt.Errorf("repository created (%s) but push failed: %w", repo.HTMLURL, err)
	}
	return repo.HTMLURL, nil
}

// gitAuth is what git needs to act as an account on some forge: the token,
// the identity to commit as, and the username the credential helper offers.
// The zero value means "use the user's own git configuration" — that is the
// BYOR path, where the app supplies no credentials at all.
type gitAuth struct {
	Token string
	Login string
	User  string // credential-helper username
	Email string
}

// gitAuthFor builds the git credentials for one forge account. GitHub and
// GitLab both accept "oauth2" as the username with the token as password;
// Gitea does plain HTTP basic and wants the real username.
func gitAuthFor(acc Account, token string) gitAuth {
	host := acc.Host
	if host == "" {
		host = "github.com"
	}
	user := "oauth2"
	if acc.Scheme == "gitea" {
		user = safeGitUser(acc.Login)
	}
	return gitAuth{
		Token: token,
		Login: acc.Login,
		User:  user,
		Email: acc.Login + "@users.noreply." + host,
	}
}

// safeGitUser strips anything that could escape the credential helper's
// shell snippet. Forge usernames never legitimately contain such characters.
func safeGitUser(login string) string {
	keep := func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.' || r == '_' || r == '-':
			return r
		}
		return -1
	}
	out := strings.Map(keep, login)
	if out == "" {
		return "oauth2"
	}
	return out
}

// runGit executes one git command with identity configured and the token
// travelling via environment to a one-shot credential helper — never on the
// command line, never persisted in .git/config.
func runGit(dir string, auth gitAuth, args ...string) error {
	var full []string
	if auth.Token == "" {
		// BYOR and other unauthenticated remotes: leave identity and
		// credentials to the user's own git configuration and helper.
		full = args
	} else {
		user := auth.User
		if user == "" {
			user = "oauth2"
		}
		helper := `!f() { echo "username=` + user + `"; echo "password=$TEMPLETRY_FORGE_TOKEN"; }; f`
		full = append([]string{
			"-c", "user.name=" + auth.Login,
			"-c", "user.email=" + auth.Email,
			"-c", "credential.helper=",
			"-c", "credential.helper=" + helper,
		}, args...)
	}
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"TEMPLETRY_FORGE_TOKEN="+auth.Token,
		"GIT_TERMINAL_PROMPT=0",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(out)))
	}
	return nil
}

// gitPublish initializes the rendered directory (if needed) and pushes it.
func gitPublish(dir, remoteURL string, auth gitAuth) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		if err := runGit(dir, auth, "init"); err != nil {
			return err
		}
	}
	if err := runGit(dir, auth, "add", "-A"); err != nil {
		return err
	}
	if err := runGit(dir, auth, "commit", "-m", "Initial commit — generated by Templetry"); err != nil && !strings.Contains(err.Error(), "nothing to commit") {
		return err
	}
	if err := runGit(dir, auth, "branch", "-M", "main"); err != nil {
		return err
	}
	_ = runGit(dir, auth, "remote", "add", "origin", remoteURL)
	return runGit(dir, auth, "push", "-u", "origin", "main")
}
