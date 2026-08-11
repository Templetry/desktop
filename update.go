package main

import (
	"fmt"

	"github.com/Templetry/engine/update"
)

// PreviewUpdate delegates to the engine's update package: re-render the
// project's template at head with the recorded inputs and diff against
// disk. Nothing is written; the preview is cached for the follow-ups.
func (a *App) PreviewUpdate(dir string) (*update.Preview, error) {
	a.mu.Lock()
	token := a.token
	a.mu.Unlock()
	p, err := update.Prepare(dir, token)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.upd = p
	a.mu.Unlock()
	return p, nil
}

// UpdateFileContent returns the NEW content of one previewed file.
func (a *App) UpdateFileContent(path string) (string, error) {
	a.mu.Lock()
	p := a.upd
	a.mu.Unlock()
	if p == nil {
		return "", fmt.Errorf("run an update preview first")
	}
	if f := p.Files.Get(path); f != nil && f.Binary {
		return "", fmt.Errorf("binary file (%d bytes)", len(f.Data))
	}
	data, err := p.FileContent(path)
	if err != nil {
		return "", err
	}
	if len(data) > 300_000 {
		return string(data[:300_000]) + "\n… (truncated)", nil
	}
	return string(data), nil
}

// ApplyUpdate writes the previewed files into the project. It never
// deletes anything; review the result with git.
func (a *App) ApplyUpdate() (int, error) {
	a.mu.Lock()
	p := a.upd
	a.upd = nil
	a.mu.Unlock()
	if p == nil {
		return 0, fmt.Errorf("run an update preview first")
	}
	return p.Apply()
}
