package main

import (
	"fmt"

	"github.com/Templetry/engine/manifest"
	"github.com/Templetry/engine/planner"
	"github.com/Templetry/engine/render"
)

// PreviewEntry is one file of an in-memory rendered preview.
type PreviewEntry struct {
	Path   string `json:"path"`
	Binary bool   `json:"binary"`
	Size   int    `json:"size"`
}

// PreviewProject renders the template in memory with the given inputs and
// returns the resulting file list. Nothing touches the disk.
func (a *App) PreviewProject(ref string, vars map[string]string, feats map[string]bool) ([]PreviewEntry, error) {
	b, err := a.fetchBundle(ref)
	if err != nil {
		return nil, err
	}
	p, err := planner.Build(b.manifest, manifest.Inputs{Variables: vars, Features: feats}, b.files)
	if err != nil {
		return nil, err
	}
	p.Source = b.source
	result, err := render.Apply(p, b.files)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.preview = result
	a.mu.Unlock()
	entries := make([]PreviewEntry, 0, result.Len())
	for _, path := range result.Paths() {
		f := result.Get(path)
		entries = append(entries, PreviewEntry{Path: path, Binary: f.Binary, Size: len(f.Data)})
	}
	return entries, nil
}

// PreviewFile returns the rendered content of one file from the last preview.
func (a *App) PreviewFile(path string) (string, error) {
	a.mu.Lock()
	pv := a.preview
	a.mu.Unlock()
	if pv == nil {
		return "", fmt.Errorf("run a preview first")
	}
	f := pv.Get(path)
	if f == nil {
		return "", fmt.Errorf("no %s in the preview", path)
	}
	if f.Binary {
		return "", fmt.Errorf("binary file (%d bytes)", len(f.Data))
	}
	if len(f.Data) > 300_000 {
		return string(f.Data[:300_000]) + "\n… (truncated)", nil
	}
	return string(f.Data), nil
}
