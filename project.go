package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Templetry/engine/manifest"
	"github.com/Templetry/engine/planner"
	"github.com/Templetry/engine/render"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// CreatedProject is the outcome of the full pipeline.
type CreatedProject struct {
	URL string `json:"url"`
	Dir string `json:"dir"`
}

type appConfig struct {
	LastParentDir    string `json:"lastParentDir"`
	DefaultParentDir string `json:"defaultParentDir"`
	DefaultOwner     string `json:"defaultOwner"`
	DefaultPrivate   bool   `json:"defaultPrivate"`
	DefaultLicense   string `json:"defaultLicense"`
	RegistryURL      string `json:"registryUrl"`
	UITheme          string `json:"uiTheme"`   // dark (default) | light
	UIAccent         string `json:"uiAccent"`  // hex; empty = brand brass
	UIDensity        string `json:"uiDensity"` // comfortable (default) | compact
	UIScale          string `json:"uiScale"`   // 0.9 | 1 | 1.1 | 1.25
	UILayout         string `json:"uiLayout"`  // auto (default) | stacked
}

// effectiveParentDir prefers the configured default over the last-used one.
func effectiveParentDir() string {
	c := loadConfig()
	if c.DefaultParentDir != "" {
		return c.DefaultParentDir
	}
	return c.LastParentDir
}

// GetSettings returns the persisted app settings.
func (a *App) GetSettings() appConfig { return loadConfig() }

// SaveSettings persists the app settings.
func (a *App) SaveSettings(c appConfig) { saveConfig(c) }

func configFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Templetry", "config.json")
}

func loadConfig() appConfig {
	var c appConfig
	if data, err := os.ReadFile(configFile()); err == nil {
		_ = json.Unmarshal(data, &c)
	}
	return c
}

func saveConfig(c appConfig) {
	path := configFile()
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.Marshal(c)
	_ = os.WriteFile(path, data, 0o644)
}

// GetLastParentDir returns the remembered parent folder for new projects.
func (a *App) GetLastParentDir() string {
	return loadConfig().LastParentDir
}

// ChooseParentDir opens the native picker, starting at the remembered folder.
func (a *App) ChooseParentDir() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choose where to create the project folder",
		DefaultDirectory: effectiveParentDir(),
	})
}

// GetOwners lists where repos can be created: the user plus their orgs.
func (a *App) GetOwners() ([]string, error) {
	a.mu.Lock()
	token, login := a.token, a.auth.Login
	a.mu.Unlock()
	if token == "" {
		return nil, fmt.Errorf("sign in first")
	}
	req, _ := http.NewRequest("GET", "https://api.github.com/user/orgs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var orgs []struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&orgs); err != nil {
		return nil, err
	}
	owners := []string{login}
	for _, o := range orgs {
		owners = append(owners, o.Login)
	}
	return owners, nil
}

// CreateFullProject runs the repo-first pipeline: create the cloud repo
// (owner = user login or an org; empty owner = local only), clone it into
// parentDir/name, render the template into the clone, commit and push.
func (a *App) CreateFullProject(ref, owner, name, description, license string, private bool, parentDir string, vars map[string]string, feats map[string]bool) (CreatedProject, error) {
	none := CreatedProject{}
	if name == "" || parentDir == "" {
		return none, fmt.Errorf("repository name and local folder are required")
	}
	target := filepath.Join(parentDir, name)
	if _, err := os.Stat(target); err == nil {
		return none, fmt.Errorf("%s already exists", target)
	}

	// Validate inputs and build the plan BEFORE touching the cloud.
	b, err := a.fetchBundle(ref)
	if err != nil {
		return none, err
	}
	p, err := planner.Build(b.manifest, manifest.Inputs{Variables: vars, Features: feats}, b.files)
	if err != nil {
		return none, err
	}
	p.Source = b.source

	a.mu.Lock()
	token, login := a.token, a.auth.Login
	a.mu.Unlock()

	htmlURL := ""
	if owner != "" {
		if token == "" {
			return none, fmt.Errorf("sign in with GitHub first")
		}
		endpoint := "https://api.github.com/user/repos"
		if owner != login {
			endpoint = "https://api.github.com/orgs/" + owner + "/repos"
		}
		payload := map[string]any{"name": name, "description": description, "private": private}
		if license != "" {
			payload["license_template"] = license
			payload["auto_init"] = true
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return none, err
		}
		var repo struct {
			HTMLURL  string `json:"html_url"`
			CloneURL string `json:"clone_url"`
			Message  string `json:"message"`
		}
		err = json.NewDecoder(resp.Body).Decode(&repo)
		resp.Body.Close()
		if err != nil {
			return none, err
		}
		if resp.StatusCode != http.StatusCreated {
			return none, fmt.Errorf("creating repository in %s: %s (HTTP %d)", owner, repo.Message, resp.StatusCode)
		}
		htmlURL = repo.HTMLURL
		if err := runGit(parentDir, token, login, "clone", repo.CloneURL, name); err != nil {
			return none, fmt.Errorf("repo created (%s) but clone failed: %w", htmlURL, err)
		}
	} else {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return none, err
		}
	}

	result, err := render.Apply(p, b.files)
	if err != nil {
		return none, err
	}
	if err := render.WriteDir(result, target); err != nil {
		return none, err
	}

	if owner != "" {
		steps := [][]string{
			{"add", "-A"},
			{"commit", "-m", "Initial project — generated by Templetry"},
			{"branch", "-M", "main"},
			{"push", "-u", "origin", "main"},
		}
		for _, s := range steps {
			if err := runGit(target, token, login, s...); err != nil {
				return none, fmt.Errorf("repo created (%s) but push failed: %w", htmlURL, err)
			}
		}
	}

	c := loadConfig()
	c.LastParentDir = parentDir
	saveConfig(c)
	return CreatedProject{URL: htmlURL, Dir: target}, nil
}
