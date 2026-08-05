package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Templetry/engine/manifest"
	"github.com/Templetry/engine/planner"
	"github.com/Templetry/engine/render"
	"github.com/Templetry/engine/source"
	"github.com/goccy/go-yaml"
)

// UpdateEntry is one file the template update would touch.
type UpdateEntry struct {
	Path   string `json:"path"`
	Status string `json:"status"` // added | modified
}

// UpdatePreview summarizes what applying the current template would change.
type UpdatePreview struct {
	Dir       string        `json:"dir"`
	Template  string        `json:"template"`
	OldCommit string        `json:"oldCommit"`
	NewCommit string        `json:"newCommit"`
	Entries   []UpdateEntry `json:"entries"`
	Unchanged int           `json:"unchanged"`
}

func normEOL(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }

// PreviewUpdate re-renders a project's template at its current head with the
// recorded inputs and diffs the result against the project on disk. Nothing
// is written; the result is cached for UpdateFileContent/ApplyUpdate.
func (a *App) PreviewUpdate(dir string) (UpdatePreview, error) {
	none := UpdatePreview{}
	data, err := os.ReadFile(filepath.Join(dir, ".templetry-answers.yml"))
	if err != nil {
		return none, fmt.Errorf("no answers file in %s", dir)
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
		return none, err
	}
	rest, ok := strings.CutPrefix(ans.Template.Source, "github.com/")
	if !ok {
		return none, fmt.Errorf("project source is not a GitHub template: %s", ans.Template.Source)
	}
	repo, right, ok := strings.Cut(rest, "@")
	if !ok {
		return none, fmt.Errorf("cannot parse source %q", ans.Template.Source)
	}
	ref, path, _ := strings.Cut(right, "/")

	a.mu.Lock()
	token := a.token
	a.mu.Unlock()

	files, err := source.FetchGitHubTarball(repo, ref, path)
	if err != nil {
		return none, err
	}
	mf := files.Get("template.yml")
	if mf == nil {
		mf = files.Get("template.yaml")
	}
	if mf == nil {
		return none, fmt.Errorf("the template no longer has a template.yml")
	}
	m, err := manifest.Load(mf.Data)
	if err != nil {
		return none, err
	}
	p, err := planner.Build(m, manifest.Inputs{Variables: ans.Variables, Features: ans.Features}, files)
	if err != nil {
		return none, err
	}
	p.Source = ans.Template.Source
	if sha, err := source.ResolveGitHubRef(repo, ref, token); err == nil {
		p.SourceCommit = sha
	}
	rendered, err := render.Apply(p, files)
	if err != nil {
		return none, err
	}

	out := UpdatePreview{
		Dir: dir, Template: ans.Template.Name,
		OldCommit: ans.Template.Commit, NewCommit: p.SourceCommit,
	}
	for _, rp := range rendered.Paths() {
		newData := rendered.Get(rp).Data
		current, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rp)))
		switch {
		case err != nil:
			out.Entries = append(out.Entries, UpdateEntry{Path: rp, Status: "added"})
		case !bytes.Equal(normEOL(current), normEOL(newData)):
			out.Entries = append(out.Entries, UpdateEntry{Path: rp, Status: "modified"})
		default:
			out.Unchanged++
		}
	}

	a.mu.Lock()
	a.updDir = dir
	a.updFiles = rendered
	a.updEntries = out.Entries
	a.mu.Unlock()
	return out, nil
}

// UpdateFileContent returns the NEW rendered content of one previewed file.
func (a *App) UpdateFileContent(path string) (string, error) {
	a.mu.Lock()
	files := a.updFiles
	a.mu.Unlock()
	if files == nil {
		return "", fmt.Errorf("run an update preview first")
	}
	f := files.Get(path)
	if f == nil {
		return "", fmt.Errorf("no %s in the update preview", path)
	}
	if f.Binary {
		return "", fmt.Errorf("binary file (%d bytes)", len(f.Data))
	}
	if len(f.Data) > 300_000 {
		return string(f.Data[:300_000]) + "\n… (truncated)", nil
	}
	return string(f.Data), nil
}

// ApplyUpdate writes the previewed added/modified files into the project.
// It never deletes anything; review the result with git.
func (a *App) ApplyUpdate() (int, error) {
	a.mu.Lock()
	dir, files, entries := a.updDir, a.updFiles, a.updEntries
	a.updDir, a.updFiles, a.updEntries = "", nil, nil
	a.mu.Unlock()
	if files == nil || dir == "" {
		return 0, fmt.Errorf("run an update preview first")
	}
	written := 0
	for _, e := range entries {
		f := files.Get(e.Path)
		if f == nil {
			continue
		}
		full := filepath.Join(dir, filepath.FromSlash(e.Path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(full, f.Data, 0o644); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}
