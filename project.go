package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	Catalogs         []CatalogEntry `json:"catalogs"`
}

// CatalogEntry is one user-defined catalog (the official one is built in).
type CatalogEntry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// effectiveParentDir prefers the configured default over the last-used one.
func effectiveParentDir() string {
	c := loadConfig()
	if c.DefaultParentDir != "" {
		return c.DefaultParentDir
	}
	return c.LastParentDir
}

// withDefaults fills empty fields so the UI always receives valid values.
func withDefaults(c appConfig) appConfig {
	if c.UITheme == "" {
		c.UITheme = "dark"
	}
	if c.UIDensity == "" {
		c.UIDensity = "comfortable"
	}
	if c.UIScale == "" {
		c.UIScale = "1"
	}
	if c.UILayout == "" {
		c.UILayout = "auto"
	}
	if c.UIAccent == "" {
		c.UIAccent = "#d9a441"
	}
	return c
}

// GetSettings returns the persisted app settings with defaults applied.
// On first run (no config file yet) new repos default to private.
func (a *App) GetSettings() appConfig {
	c := loadConfig()
	if _, err := os.Stat(configFile()); err != nil {
		c.DefaultPrivate = true
	}
	return withDefaults(c)
}

// ExportSettings writes the settings to a user-chosen JSON file. The GitHub
// token lives in the OS keyring, never in settings — exports are shareable.
func (a *App) ExportSettings() (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export Templetry settings",
		DefaultFilename: "templetry-settings.json",
		Filters:         []runtime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	data, err := json.MarshalIndent(loadConfig(), "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// ImportSettings loads settings from a user-chosen JSON file, persists them
// and returns them so the UI can apply immediately.
func (a *App) ImportSettings() (appConfig, error) {
	current := loadConfig()
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Import Templetry settings",
		Filters: []runtime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return current, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return current, err
	}
	var c appConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return current, fmt.Errorf("not a valid settings file: %w", err)
	}
	saveConfig(c)
	return withDefaults(c), nil
}

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

// webURLFor turns a clone URL into something a browser can open: strips
// the .git suffix and rewrites the scp-like SSH form. Anything else is
// returned untouched (the UI only offers it as a link when it is http).
func webURLFor(clone string) string {
	u := strings.TrimSuffix(strings.TrimSpace(clone), ".git")
	if rest, ok := strings.CutPrefix(u, "git@"); ok {
		if host, path, found := strings.Cut(rest, ":"); found {
			return "https://" + host + "/" + path
		}
	}
	if strings.HasPrefix(u, "ssh://git@") {
		return "https://" + strings.TrimPrefix(u, "ssh://git@")
	}
	return u
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
func (a *App) CreateFullProject(cat, ref, owner, name, description, license string, private bool, parentDir string, vars map[string]string, feats map[string]bool) (CreatedProject, error) {
	return a.createProject(cat, ref, owner, name, description, license, private, parentDir, "", vars, feats)
}

// CreateProjectOnRemote is the BYOR pipeline (ADR-0009, ADR-0015): the user
// creates an empty repository on ANY git host and pastes its URL. Templetry
// renders locally, initializes the repo and pushes — no forge API, so every
// host works: GitLab, Gitea, Forgejo, Bitbucket, self-hosted, plain SSH.
func (a *App) CreateProjectOnRemote(cat, ref, name, remoteURL, parentDir string, vars map[string]string, feats map[string]bool) (CreatedProject, error) {
	if remoteURL == "" {
		return CreatedProject{}, fmt.Errorf("paste the URL of an empty repository first")
	}
	return a.createProject(cat, ref, "", name, "", "", false, parentDir, remoteURL, vars, feats)
}

func (a *App) createProject(cat, ref, owner, name, description, license string, private bool, parentDir, remoteURL string, vars map[string]string, feats map[string]bool) (CreatedProject, error) {
	none := CreatedProject{}
	if name == "" || parentDir == "" {
		return none, fmt.Errorf("repository name and local folder are required")
	}
	target := filepath.Join(parentDir, name)
	if _, err := os.Stat(target); err == nil {
		return none, fmt.Errorf("%s already exists", target)
	}

	// Validate inputs and build the plan BEFORE touching the cloud.
	b, err := a.fetchBundle(cat, ref)
	if err != nil {
		return none, err
	}
	p, err := planner.Build(b.manifest, manifest.Inputs{Variables: vars, Features: feats}, b.files)
	if err != nil {
		return none, err
	}
	p.Source = b.source
	p.SourceCommit = b.commit

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

	switch {
	case owner != "":
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
	case remoteURL != "":
		// BYOR: no forge API — git credentials come from the user's own
		// helper (manager, SSH agent, netrc), never from the app.
		if err := gitPublish(target, remoteURL, "", ""); err != nil {
			return none, fmt.Errorf("project rendered in %s but push to %s failed: %w", target, remoteURL, err)
		}
		htmlURL = webURLFor(remoteURL)
	}

	c := loadConfig()
	c.LastParentDir = parentDir
	saveConfig(c)
	return CreatedProject{URL: htmlURL, Dir: target}, nil
}
