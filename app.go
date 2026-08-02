package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/Templetry/engine/catalog"
	"github.com/Templetry/engine/manifest"
	"github.com/Templetry/engine/planner"
	"github.com/Templetry/engine/render"
	"github.com/Templetry/engine/source"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App drives the embedded Templetry engine (ADR-0012).
type App struct {
	ctx   context.Context
	mu    sync.Mutex
	reg   *catalog.Registry
	cache map[string]*bundle
}

type bundle struct {
	files    *source.FileSet
	manifest *manifest.Manifest
	source   string
}

func NewApp() *App {
	return &App{cache: map[string]*bundle{}}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetCatalog fetches the official registry (cached per session).
func (a *App) GetCatalog() (*catalog.Registry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.reg != nil {
		return a.reg, nil
	}
	resp, err := http.Get(catalog.DefaultRegistryURL)
	if err != nil {
		return nil, fmt.Errorf("fetching catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching catalog: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	reg, err := catalog.Parse(data)
	if err != nil {
		return nil, err
	}
	a.reg = reg
	return reg, nil
}

// fetchBundle downloads and caches a form's template.
func (a *App) fetchBundle(ref string) (*bundle, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if b, ok := a.cache[ref]; ok {
		return b, nil
	}
	if a.reg == nil {
		return nil, fmt.Errorf("catalog not loaded yet")
	}
	parent, form, err := a.reg.Resolve(ref)
	if err != nil {
		return nil, err
	}
	files, err := source.FetchGitHubTarball(parent.Repo, parent.Ref, form.Path)
	if err != nil {
		return nil, err
	}
	f := files.Get("template.yml")
	if f == nil {
		f = files.Get("template.yaml")
	}
	if f == nil {
		return nil, fmt.Errorf("form %s has no template.yml", ref)
	}
	m, err := manifest.Load(f.Data)
	if err != nil {
		return nil, err
	}
	b := &bundle{
		files:    files,
		manifest: m,
		source:   fmt.Sprintf("github.com/%s@%s/%s", parent.Repo, parent.Ref, form.Path),
	}
	a.cache[ref] = b
	return b, nil
}

// GetTemplate returns a form's manifest — the dynamic form definition.
func (a *App) GetTemplate(ref string) (*manifest.Manifest, error) {
	b, err := a.fetchBundle(ref)
	if err != nil {
		return nil, err
	}
	return b.manifest, nil
}

// PlanProject returns the human-readable dry-run for the given inputs.
func (a *App) PlanProject(ref string, vars map[string]string, feats map[string]bool) (string, error) {
	b, err := a.fetchBundle(ref)
	if err != nil {
		return "", err
	}
	p, err := planner.Build(b.manifest, manifest.Inputs{Variables: vars, Features: feats}, b.files)
	if err != nil {
		return "", err
	}
	return p.Describe(), nil
}

// CreateProject renders the form into outDir.
func (a *App) CreateProject(ref, outDir string, vars map[string]string, feats map[string]bool) (string, error) {
	if outDir == "" {
		return "", fmt.Errorf("choose an output directory first")
	}
	b, err := a.fetchBundle(ref)
	if err != nil {
		return "", err
	}
	p, err := planner.Build(b.manifest, manifest.Inputs{Variables: vars, Features: feats}, b.files)
	if err != nil {
		return "", err
	}
	p.Source = b.source
	if entries, err := os.ReadDir(outDir); err == nil && len(entries) > 0 {
		return "", fmt.Errorf("output directory %s is not empty", outDir)
	}
	result, err := render.Apply(p, b.files)
	if err != nil {
		return "", err
	}
	if err := render.WriteDir(result, outDir); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s → %s (%d files)", ref, outDir, result.Len()), nil
}

// ChooseOutputDir opens the native directory picker.
func (a *App) ChooseOutputDir() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choose the output directory",
	})
}
