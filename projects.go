package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

// LocalProject is one Templetry-born project found on disk.
type LocalProject struct {
	Dir       string            `json:"dir"`
	Name      string            `json:"name"`
	Template  string            `json:"template"`
	Source    string            `json:"source"`
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
			} `yaml:"template"`
			Variables map[string]string `yaml:"variables"`
			Features  map[string]bool   `yaml:"features"`
		}
		if err := yaml.Unmarshal(data, &ans); err != nil {
			continue
		}
		out = append(out, LocalProject{
			Dir: dir, Name: e.Name(),
			Template: ans.Template.Name, Source: ans.Template.Source,
			Variables: ans.Variables, Features: ans.Features,
		})
	}
	return out, nil
}

// OpenFolder shows a project directory in the system file explorer.
func (a *App) OpenFolder(dir string) {
	_ = exec.Command("explorer", dir).Start()
}
