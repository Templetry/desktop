package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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

	// Base render: the template at the recorded commit, same inputs — the
	// third band for real merges when both sides touched a file.
	var baseRendered *source.FileSet
	if ans.Template.Commit != "" {
		if baseFiles, err := source.FetchGitHubTarball(repo, ans.Template.Commit, path); err == nil {
			if bmf := baseFiles.Get("template.yml"); bmf != nil {
				if bm, err := manifest.Load(bmf.Data); err == nil {
					if bp, err := planner.Build(bm, manifest.Inputs{Variables: ans.Variables, Features: ans.Features}, baseFiles); err == nil {
						baseRendered, _ = render.Apply(bp, baseFiles)
					}
				}
			}
		}
	}

	out := UpdatePreview{
		Dir: dir, Template: ans.Template.Name,
		OldCommit: ans.Template.Commit, NewCommit: p.SourceCommit,
	}
	merged := map[string][]byte{}
	for _, rp := range rendered.Paths() {
		newData := normEOL(rendered.Get(rp).Data)
		current, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rp)))
		if err != nil {
			out.Entries = append(out.Entries, UpdateEntry{Path: rp, Status: "added"})
			continue
		}
		cur := normEOL(current)
		if bytes.Equal(cur, newData) {
			out.Unchanged++
			continue
		}
		var base []byte
		if baseRendered != nil {
			if bf := baseRendered.Get(rp); bf != nil {
				base = normEOL(bf.Data)
			}
		}
		if base != nil && bytes.Equal(cur, base) {
			// User never touched it: safe overwrite.
			out.Entries = append(out.Entries, UpdateEntry{Path: rp, Status: "modified"})
			continue
		}
		// Both sides moved (or no base known): real three-way merge.
		m, conflicts, err := gitMergeFile(cur, base, newData)
		if err != nil {
			out.Entries = append(out.Entries, UpdateEntry{Path: rp, Status: "conflict"})
			continue
		}
		merged[rp] = m
		status := "merged"
		if conflicts > 0 {
			status = "conflict"
		}
		out.Entries = append(out.Entries, UpdateEntry{Path: rp, Status: status})
	}

	a.mu.Lock()
	a.updDir = dir
	a.updFiles = rendered
	a.updEntries = out.Entries
	a.updMerged = merged
	a.mu.Unlock()
	return out, nil
}

// gitMergeFile three-way merges via git merge-file; returns merged content
// and the number of conflicts.
func gitMergeFile(ours, base, theirs []byte) ([]byte, int, error) {
	tmp, err := os.MkdirTemp("", "templetry-merge")
	if err != nil {
		return nil, 0, err
	}
	defer os.RemoveAll(tmp)
	o, b, t := filepath.Join(tmp, "ours"), filepath.Join(tmp, "base"), filepath.Join(tmp, "theirs")
	if err := os.WriteFile(o, ours, 0o644); err != nil {
		return nil, 0, err
	}
	if err := os.WriteFile(b, base, 0o644); err != nil {
		return nil, 0, err
	}
	if err := os.WriteFile(t, theirs, 0o644); err != nil {
		return nil, 0, err
	}
	cmd := exec.Command("git", "merge-file", "-p", "-L", "yours", "-L", "template-old", "-L", "template-new", o, b, t)
	out, err := cmd.Output()
	conflicts := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() > 0 {
			conflicts = ee.ExitCode()
		} else {
			return nil, 0, err
		}
	}
	return out, conflicts, nil
}

// UpdateFileContent returns the NEW rendered content of one previewed file.
func (a *App) UpdateFileContent(path string) (string, error) {
	a.mu.Lock()
	files := a.updFiles
	if m, ok := a.updMerged[path]; ok {
		a.mu.Unlock()
		return string(m), nil
	}
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
	dir, files, entries, mergedData := a.updDir, a.updFiles, a.updEntries, a.updMerged
	a.updDir, a.updFiles, a.updEntries, a.updMerged = "", nil, nil, nil
	a.mu.Unlock()
	if files == nil || dir == "" {
		return 0, fmt.Errorf("run an update preview first")
	}
	written := 0
	for _, e := range entries {
		data := []byte(nil)
		if m, ok := mergedData[e.Path]; ok {
			data = m
		} else if f := files.Get(e.Path); f != nil {
			data = f.Data
		} else {
			continue
		}
		full := filepath.Join(dir, filepath.FromSlash(e.Path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}
