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
	"github.com/Templetry/engine/update"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App drives the embedded Templetry engine (ADR-0012).
type App struct {
	ctx   context.Context
	mu    sync.Mutex
	regs  map[string]*catalog.Registry
	cache map[string]*bundle

	token        string
	auth         AuthStatus
	authRestored bool
	preview      *source.FileSet

	upd *update.Preview
}

type bundle struct {
	files    *source.FileSet
	manifest *manifest.Manifest
	source   string
	commit   string
}

func NewApp() *App {
	return &App{cache: map[string]*bundle{}, regs: map[string]*catalog.Registry{}}
}

// OfficialCatalogName labels the built-in Templetry catalog.
const OfficialCatalogName = "Templetry"

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// LoadedCatalog is one catalog as the sidebar renders it.
type LoadedCatalog struct {
	Name     string           `json:"name"`
	Official bool             `json:"official"`
	Error    string           `json:"error,omitempty"`
	Parents  []catalog.Parent `json:"parents,omitempty"`
}

func fetchRegistry(location string) (*catalog.Registry, error) {
	if data, err := os.ReadFile(location); err == nil {
		return catalog.Parse(data)
	}
	resp, err := http.Get(location)
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
	return catalog.Parse(data)
}

// GetCatalogs loads the official catalog plus every user-defined one.
// Per-catalog failures are reported, never fatal.
func (a *App) GetCatalogs() []LoadedCatalog {
	cfg := loadConfig()
	sources := []struct {
		name, url string
		official  bool
	}{{OfficialCatalogName, catalog.DefaultRegistryURL, true}}
	for _, c := range cfg.Catalogs {
		if c.Name != "" && c.URL != "" && c.Name != OfficialCatalogName {
			sources = append(sources, struct {
				name, url string
				official  bool
			}{c.Name, c.URL, false})
		}
	}
	out := make([]LoadedCatalog, 0, len(sources))
	for _, s := range sources {
		reg, err := fetchRegistry(s.url)
		if err != nil {
			out = append(out, LoadedCatalog{Name: s.name, Official: s.official, Error: err.Error()})
			continue
		}
		a.mu.Lock()
		a.regs[s.name] = reg
		a.mu.Unlock()
		out = append(out, LoadedCatalog{Name: s.name, Official: s.official, Parents: reg.Parents})
	}
	return out
}

// fetchBundle downloads and caches a form's template from a named catalog.
func (a *App) fetchBundle(cat, ref string) (*bundle, error) {
	key := cat + "::" + ref
	a.mu.Lock()
	defer a.mu.Unlock()
	if b, ok := a.cache[key]; ok {
		return b, nil
	}
	reg := a.regs[cat]
	if reg == nil {
		return nil, fmt.Errorf("catalog %q not loaded yet", cat)
	}
	parent, form, err := reg.Resolve(ref)
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
	commit := ""
	if sha, err := source.ResolveGitHubRef(parent.Repo, parent.Ref, a.token); err == nil {
		commit = sha
	}
	b := &bundle{
		files:    files,
		manifest: m,
		source:   fmt.Sprintf("github.com/%s@%s/%s", parent.Repo, parent.Ref, form.Path),
		commit:   commit,
	}
	a.cache[key] = b
	return b, nil
}

// GetTemplate returns a form's manifest — the dynamic form definition.
func (a *App) GetTemplate(cat, ref string) (*manifest.Manifest, error) {
	b, err := a.fetchBundle(cat, ref)
	if err != nil {
		return nil, err
	}
	return b.manifest, nil
}

// PlanProject returns the human-readable dry-run for the given inputs.
func (a *App) PlanProject(cat, ref string, vars map[string]string, feats map[string]bool) (string, error) {
	b, err := a.fetchBundle(cat, ref)
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
func (a *App) CreateProject(cat, ref, outDir string, vars map[string]string, feats map[string]bool) (string, error) {
	if outDir == "" {
		return "", fmt.Errorf("choose an output directory first")
	}
	b, err := a.fetchBundle(cat, ref)
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
