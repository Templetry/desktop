package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Templetry/engine/source"
	"github.com/goccy/go-yaml"
)

// LocalProject is one Templetry-born project found on disk.
type LocalProject struct {
	Dir       string            `json:"dir"`
	Name      string            `json:"name"`
	Template  string            `json:"template"`
	Source    string            `json:"source"`
	Commit    string            `json:"commit"`
	Variables map[string]string `json:"variables"`
	Features  map[string]bool   `json:"features"`
}

// ScanProjects walks the repositories folder looking for
// .templetry-answers.yml files — the provenance record every render writes.
func (a *App) ScanProjects() ([]LocalProject, error) {
	parent := effectiveParentDir()
	if parent == "" {
		return nil, fmt.Errorf("set your repositories folder in Settings first")
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil, err
	}
	out := []LocalProject{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(parent, e.Name())
		data, err := os.ReadFile(filepath.Join(dir, ".templetry-answers.yml"))
		if err != nil {
			continue
		}
		var ans struct {
			Template struct {
				Name   string `yaml:"name"`
				Source string `yaml:"source"`
				Commit string `yaml:"commit"`
			} `yaml:"template"`
			Variables map[string]string `yaml:"variables"`
			Features  map[string]bool   `yaml:"features"`
		}
		if err := yaml.Unmarshal(data, &ans); err != nil {
			continue
		}
		out = append(out, LocalProject{
			Dir: dir, Name: e.Name(),
			Template: ans.Template.Name, Source: ans.Template.Source, Commit: ans.Template.Commit,
			Variables: ans.Variables, Features: ans.Features,
		})
	}
	return out, nil
}

// Drift marks a project whose template moved past the recorded commit.
type Drift struct {
	Dir    string `json:"dir"`
	Latest string `json:"latest"`
}

// CheckDrift compares each project's recorded template commit against the
// template's current head. One API call per distinct repo@ref.
func (a *App) CheckDrift() ([]Drift, error) {
	projects, err := a.ScanProjects()
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	token := a.token
	a.mu.Unlock()
	cache := map[string]string{}
	out := []Drift{}
	for _, p := range projects {
		if p.Commit == "" || !strings.HasPrefix(p.Source, "github.com/") {
			continue
		}
		rest := strings.TrimPrefix(p.Source, "github.com/")
		repo, right, ok := strings.Cut(rest, "@")
		if !ok {
			continue
		}
		ref, _, _ := strings.Cut(right, "/")
		key := repo + "@" + ref
		latest, seen := cache[key]
		if !seen {
			latest, err = source.ResolveGitHubRef(repo, ref, token)
			if err != nil {
				continue
			}
			cache[key] = latest
		}
		if latest != p.Commit {
			out = append(out, Drift{Dir: p.Dir, Latest: latest})
		}
	}
	return out, nil
}

// OpenFolder shows a project directory in the system file explorer.
func (a *App) OpenFolder(dir string) {
	_ = exec.Command("explorer", dir).Start()
}
