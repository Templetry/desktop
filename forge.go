package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/zalando/go-keyring"
)

// Account is one signed-in forge account. Tokens never live here — they
// stay in the OS keyring under "<scheme>@<host>" (ADR-0009: login and
// forge credentials are separate concerns).
type Account struct {
	Scheme string `json:"scheme"` // github | gitlab | gitea
	Host   string `json:"host"`
	Login  string `json:"login"`
	Avatar string `json:"avatar,omitempty"`
}

// Key identifies the account in the keyring and in owner references.
func (a Account) Key() string { return a.Scheme + "@" + a.Host }

// Label is what the UI shows.
func (a Account) Label() string { return a.Login + " (" + a.Host + ")" }

// forgeAPI is the base API URL for a host.
func forgeAPI(scheme, host string) string {
	switch scheme {
	case "gitlab":
		return "https://" + host + "/api/v4"
	case "gitea":
		return "https://" + host + "/api/v1"
	default:
		return "https://api." + host
	}
}

// forgeAuth is the header a token travels in, per forge.
func forgeAuth(scheme, token string) (string, string) {
	switch scheme {
	case "gitlab":
		// Bearer, not PRIVATE-TOKEN. GitLab accepts a personal access token
		// either way, but an OAuth access token only as Bearer — and this
		// function cannot tell the two apart, nor should it have to.
		return "Authorization", "Bearer " + token
	case "gitea":
		return "Authorization", "token " + token
	default:
		return "Authorization", "Bearer " + token
	}
}

// forgeDo performs an authenticated API call and decodes the JSON body.
func forgeDo(method, u, scheme, token string, body, out any) error {
	var rdr *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(data)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, u, rdr)
	if err != nil {
		return err
	}
	// An empty token means no header at all, not an empty one: forges reject
	// a blank credential where they would have served a public repository.
	if token != "" {
		k, v := forgeAuth(scheme, token)
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var msg struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&msg)
		detail := msg.Message
		if detail == "" {
			detail = msg.Error
		}
		if detail == "" {
			detail = resp.Status
		}
		return fmt.Errorf("%s: %s", u, detail)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// forgeWhoami validates a token and returns the account it belongs to.
func forgeWhoami(scheme, host, token string) (Account, error) {
	acc := Account{Scheme: scheme, Host: host}
	base := forgeAPI(scheme, host)
	switch scheme {
	case "gitlab":
		var u struct {
			Username  string `json:"username"`
			AvatarURL string `json:"avatar_url"`
		}
		if err := forgeDo("GET", base+"/user", scheme, token, nil, &u); err != nil {
			return acc, err
		}
		acc.Login, acc.Avatar = u.Username, u.AvatarURL
	case "gitea":
		var u struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		}
		if err := forgeDo("GET", base+"/user", scheme, token, nil, &u); err != nil {
			return acc, err
		}
		acc.Login, acc.Avatar = u.Login, u.AvatarURL
	default:
		var u struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		}
		if err := forgeDo("GET", base+"/user", scheme, token, nil, &u); err != nil {
			return acc, err
		}
		acc.Login, acc.Avatar = u.Login, u.AvatarURL
	}
	if acc.Login == "" {
		return acc, fmt.Errorf("the token is valid but returned no user")
	}
	return acc, nil
}

// forgeOwners lists where this account can create repositories: the user
// plus the groups/organizations they can write to.
func forgeOwners(acc Account, token string) ([]string, error) {
	base := forgeAPI(acc.Scheme, acc.Host)
	owners := []string{acc.Login}
	switch acc.Scheme {
	case "gitlab":
		var groups []struct {
			FullPath string `json:"full_path"`
		}
		// min_access_level 30 = Developer, the floor for creating projects.
		if err := forgeDo("GET", base+"/groups?min_access_level=30&per_page=100", acc.Scheme, token, nil, &groups); err != nil {
			return owners, nil // degrade: personal namespace still works
		}
		for _, g := range groups {
			owners = append(owners, g.FullPath)
		}
	case "gitea":
		var orgs []struct {
			Username string `json:"username"`
		}
		if err := forgeDo("GET", base+"/user/orgs?limit=100", acc.Scheme, token, nil, &orgs); err != nil {
			return owners, nil
		}
		for _, o := range orgs {
			owners = append(owners, o.Username)
		}
	default:
		var orgs []struct {
			Login string `json:"login"`
		}
		if err := forgeDo("GET", base+"/user/orgs", acc.Scheme, token, nil, &orgs); err != nil {
			return owners, nil
		}
		for _, o := range orgs {
			owners = append(owners, o.Login)
		}
	}
	return owners, nil
}

// forgeCreateRepo creates a repository and returns its clone and web URLs.
func forgeCreateRepo(acc Account, token, owner, name, description string, private bool) (clone, html string, err error) {
	base := forgeAPI(acc.Scheme, acc.Host)
	switch acc.Scheme {
	case "gitlab":
		payload := map[string]any{
			"name": name, "path": name, "description": description,
			"visibility": visibility(private), "initialize_with_readme": false,
		}
		if owner != acc.Login {
			id, err := gitlabNamespaceID(acc, token, owner)
			if err != nil {
				return "", "", err
			}
			payload["namespace_id"] = id
		}
		var out struct {
			HTTPURL string `json:"http_url_to_repo"`
			WebURL  string `json:"web_url"`
		}
		if err := forgeDo("POST", base+"/projects", acc.Scheme, token, payload, &out); err != nil {
			return "", "", err
		}
		return out.HTTPURL, out.WebURL, nil
	case "gitea":
		endpoint := base + "/user/repos"
		if owner != acc.Login {
			endpoint = base + "/orgs/" + owner + "/repos"
		}
		payload := map[string]any{"name": name, "description": description, "private": private, "auto_init": false}
		var out struct {
			CloneURL string `json:"clone_url"`
			HTMLURL  string `json:"html_url"`
		}
		if err := forgeDo("POST", endpoint, acc.Scheme, token, payload, &out); err != nil {
			return "", "", err
		}
		return out.CloneURL, out.HTMLURL, nil
	default:
		endpoint := base + "/user/repos"
		if owner != acc.Login {
			endpoint = base + "/orgs/" + owner + "/repos"
		}
		payload := map[string]any{"name": name, "description": description, "private": private}
		var out struct {
			CloneURL string `json:"clone_url"`
			HTMLURL  string `json:"html_url"`
		}
		if err := forgeDo("POST", endpoint, acc.Scheme, token, payload, &out); err != nil {
			return "", "", err
		}
		return out.CloneURL, out.HTMLURL, nil
	}
}

func visibility(private bool) string {
	if private {
		return "private"
	}
	return "public"
}

// gitlabNamespaceID resolves a group path to the numeric id GitLab wants.
func gitlabNamespaceID(acc Account, token, path string) (int, error) {
	var found []struct {
		ID       int    `json:"id"`
		FullPath string `json:"full_path"`
	}
	u := forgeAPI(acc.Scheme, acc.Host) + "/namespaces?search=" + url.QueryEscape(path)
	if err := forgeDo("GET", u, acc.Scheme, token, nil, &found); err != nil {
		return 0, err
	}
	for _, n := range found {
		if strings.EqualFold(n.FullPath, path) {
			return n.ID, nil
		}
	}
	return 0, fmt.Errorf("group %q not found on %s", path, acc.Host)
}

// forgeListRepos lists the account's repositories, most recent first.
func forgeListRepos(acc Account, token string) ([]Repo, error) {
	base := forgeAPI(acc.Scheme, acc.Host)
	out := []Repo{}
	switch acc.Scheme {
	case "gitlab":
		var projects []struct {
			Name              string `json:"name"`
			PathWithNamespace string `json:"path_with_namespace"`
			Description       string `json:"description"`
			WebURL            string `json:"web_url"`
			HTTPURL           string `json:"http_url_to_repo"`
			Visibility        string `json:"visibility"`
			LastActivityAt    string `json:"last_activity_at"`
			Archived          bool   `json:"archived"`
			DefaultBranch     string `json:"default_branch"`
			Namespace         struct {
				FullPath  string `json:"full_path"`
				AvatarURL string `json:"avatar_url"`
			} `json:"namespace"`
		}
		u := base + "/projects?membership=true&order_by=last_activity_at&per_page=100&simple=false"
		if err := forgeDo("GET", u, acc.Scheme, token, nil, &projects); err != nil {
			return nil, err
		}
		for _, p := range projects {
			out = append(out, Repo{
				Name: p.Name, FullName: p.PathWithNamespace, Owner: p.Namespace.FullPath,
				Description: p.Description, HTMLURL: p.WebURL, CloneURL: p.HTTPURL,
				Private: p.Visibility != "public", UpdatedAt: p.LastActivityAt,
				Archived: p.Archived, AvatarURL: p.Namespace.AvatarURL, Forge: acc.Key(),
				DefaultBranch: p.DefaultBranch,
			})
		}
	case "gitea":
		var repos []struct {
			Name     string `json:"name"`
			FullName string `json:"full_name"`
			Desc     string `json:"description"`
			HTMLURL  string `json:"html_url"`
			CloneURL string `json:"clone_url"`
			Private  bool   `json:"private"`
			Updated  string `json:"updated_at"`
			Archived bool   `json:"archived"`
			Language string `json:"language"`
			Branch   string `json:"default_branch"`
			Owner    struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
			} `json:"owner"`
		}
		if err := forgeDo("GET", base+"/user/repos?limit=100", acc.Scheme, token, nil, &repos); err != nil {
			return nil, err
		}
		for _, r := range repos {
			out = append(out, Repo{
				Name: r.Name, FullName: r.FullName, Owner: r.Owner.Login,
				Description: r.Desc, HTMLURL: r.HTMLURL, CloneURL: r.CloneURL,
				Private: r.Private, Language: r.Language, UpdatedAt: r.Updated,
				Archived: r.Archived, AvatarURL: r.Owner.AvatarURL, Forge: acc.Key(),
				DefaultBranch: r.Branch,
			})
		}
	}
	return out, nil
}

// --- Account storage -------------------------------------------------

// accountToken is what every forge call and every git push asks for. It
// goes through the credential layer, so an OAuth grant that is about to
// expire is renewed here rather than failing somewhere less explainable.
func accountToken(acc Account) (string, error) {
	return accountCredential(acc)
}

// GetAccounts returns every signed-in forge account: the GitHub OAuth
// session (when present) plus token accounts added in Settings.
func (a *App) GetAccounts() []Account {
	out := []Account{}
	a.mu.Lock()
	ghToken, ghAuth := a.token, a.auth
	a.mu.Unlock()
	if ghToken != "" && ghAuth.Login != "" {
		out = append(out, Account{Scheme: "github", Host: "github.com", Login: ghAuth.Login, Avatar: ghAuth.Avatar})
	}
	for _, acc := range loadConfig().Accounts {
		if _, err := accountToken(acc); err == nil {
			out = append(out, acc)
		}
	}
	return out
}

// AddAccount validates a personal access token against the forge and
// stores it in the OS keyring. Supported schemes: gitlab, gitea (which
// also covers Forgejo and Codeberg). GitHub uses the OAuth device flow.
func (a *App) AddAccount(scheme, host, token string) (Account, error) {
	scheme, host, token = strings.TrimSpace(scheme), strings.TrimSpace(host), strings.TrimSpace(token)
	if scheme != "gitlab" && scheme != "gitea" {
		return Account{}, fmt.Errorf("scheme must be gitlab or gitea (GitHub signs in from the top bar)")
	}
	if host == "" || token == "" {
		return Account{}, fmt.Errorf("host and token are required")
	}
	host = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://"), "/")
	acc, err := forgeWhoami(scheme, host, token)
	if err != nil {
		return Account{}, fmt.Errorf("could not sign in to %s: %w", host, err)
	}
	if err := keyring.Set(keyringService, acc.Key(), token); err != nil {
		return Account{}, err
	}
	c := loadConfig()
	replaced := false
	for i, existing := range c.Accounts {
		if existing.Key() == acc.Key() {
			c.Accounts[i] = acc
			replaced = true
		}
	}
	if !replaced {
		c.Accounts = append(c.Accounts, acc)
	}
	saveConfig(c)
	return acc, nil
}

// RemoveAccount forgets an account and deletes its token.
func (a *App) RemoveAccount(key string) {
	_ = keyring.Delete(keyringService, key)
	c := loadConfig()
	kept := c.Accounts[:0]
	for _, acc := range c.Accounts {
		if acc.Key() != key {
			kept = append(kept, acc)
		}
	}
	c.Accounts = kept
	saveConfig(c)
}

// OwnerOption is one destination the Build view can create a repo in.
type OwnerOption struct {
	Key   string `json:"key"`   // "<scheme>@<host>/<owner>"
	Label string `json:"label"` // "owner (host)"
	Forge string `json:"forge"` // account key
}

// GetOwnerOptions lists every namespace across every signed-in account.
func (a *App) GetOwnerOptions() []OwnerOption {
	out := []OwnerOption{}
	for _, acc := range a.GetAccounts() {
		token, err := accountToken(acc)
		if err != nil && acc.Scheme == "github" {
			a.mu.Lock()
			token = a.token
			a.mu.Unlock()
		} else if err != nil {
			continue
		}
		owners, err := forgeOwners(acc, token)
		if err != nil || len(owners) == 0 {
			// Groups could not be read. The account's own namespace is still
			// known and still a valid destination — offering it beats the
			// account disappearing from the list with no explanation.
			owners = []string{acc.Login}
		}
		for _, o := range owners {
			out = append(out, OwnerOption{
				Key:   acc.Key() + "/" + o,
				Label: o + " (" + acc.Host + ")",
				Forge: acc.Key(),
			})
		}
	}
	return out
}

// resolveOwnerKey splits "<scheme>@<host>/<owner>" into its account and
// owner, returning the account's token.
func (a *App) resolveOwnerKey(key string) (Account, string, string, error) {
	accKey, owner, ok := strings.Cut(key, "/")
	if !ok {
		return Account{}, "", "", fmt.Errorf("malformed destination %q", key)
	}
	for _, acc := range a.GetAccounts() {
		if acc.Key() != accKey {
			continue
		}
		if acc.Scheme == "github" {
			a.mu.Lock()
			token := a.token
			a.mu.Unlock()
			return acc, owner, token, nil
		}
		token, err := accountToken(acc)
		if err != nil {
			return Account{}, "", "", fmt.Errorf("no stored token for %s — sign in again", accKey)
		}
		return acc, owner, token, nil
	}
	return Account{}, "", "", fmt.Errorf("no signed-in account for %s", accKey)
}
