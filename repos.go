package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Repo is the slim listing the projects view renders.
type Repo struct {
	Name        string `json:"name"`
	FullName    string `json:"fullName"`
	Owner       string `json:"owner"`
	Description string `json:"description"`
	HTMLURL     string `json:"htmlUrl"`
	CloneURL    string `json:"cloneUrl"`
	Private     bool   `json:"private"`
	Language    string `json:"language"`
	UpdatedAt   string `json:"updatedAt"`
	Archived    bool   `json:"archived"`
	AvatarURL   string `json:"avatarUrl"`
}

// ListRepos returns the user's repositories across personal account and orgs,
// most recently updated first.
func (a *App) ListRepos() ([]Repo, error) {
	a.mu.Lock()
	token := a.token
	a.mu.Unlock()
	if token == "" {
		return nil, fmt.Errorf("sign in first")
	}
	var out []Repo
	for page := 1; page <= 3; page++ {
		url := fmt.Sprintf("https://api.github.com/user/repos?affiliation=owner,organization_member&sort=updated&per_page=100&page=%d", page)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		var batch []struct {
			Name        string `json:"name"`
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			HTMLURL     string `json:"html_url"`
			CloneURL    string `json:"clone_url"`
			Private     bool   `json:"private"`
			Language    string `json:"language"`
			UpdatedAt   string `json:"updated_at"`
			Archived    bool   `json:"archived"`
			Owner       struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
			} `json:"owner"`
		}
		err = json.NewDecoder(resp.Body).Decode(&batch)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		for _, r := range batch {
			out = append(out, Repo{
				Name: r.Name, FullName: r.FullName, Owner: r.Owner.Login,
				Description: r.Description, HTMLURL: r.HTMLURL, CloneURL: r.CloneURL,
				Private: r.Private, Language: r.Language, UpdatedAt: r.UpdatedAt,
				Archived: r.Archived, AvatarURL: r.Owner.AvatarURL,
			})
		}
		if len(batch) < 100 {
			break
		}
	}
	return out, nil
}

// OpenRepo opens a repository page in the default browser.
func (a *App) OpenRepo(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// CloneRepo clones a repository into the remembered parent folder and
// returns the local path.
func (a *App) CloneRepo(cloneURL, name string) (string, error) {
	a.mu.Lock()
	token, login := a.token, a.auth.Login
	a.mu.Unlock()
	if token == "" {
		return "", fmt.Errorf("sign in first")
	}
	parent := effectiveParentDir()
	if parent == "" {
		return "", fmt.Errorf("set your default repositories folder in Settings first")
	}
	target := filepath.Join(parent, name)
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("%s already exists", target)
	}
	if err := runGit(parent, token, login, "clone", cloneURL, name); err != nil {
		return "", err
	}
	return target, nil
}
