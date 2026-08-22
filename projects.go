package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Templetry/engine/answers"
	"github.com/Templetry/engine/catalog"
	"github.com/Templetry/engine/manifest"
	"github.com/Templetry/engine/piece"
	"github.com/Templetry/engine/source"
	"github.com/goccy/go-yaml"
)

// LocalProject is one project found on disk: a Templetry-born render
// (kind "templetry") or a plain git repository (kind "git").
type LocalProject struct {
	Dir       string            `json:"dir"`
	Name      string            `json:"name"`
	Rel       string            `json:"rel"`
	Kind      string            `json:"kind"`
	Remote    string            `json:"remote,omitempty"`
	Host      string            `json:"host,omitempty"`
	Owner     string            `json:"owner,omitempty"`
	AvatarURL string            `json:"avatarUrl,omitempty"`
	Branch    string            `json:"branch,omitempty"`
	Template  string            `json:"template,omitempty"`
	Source    string            `json:"source,omitempty"`
	Commit    string            `json:"commit,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	Features  map[string]bool   `json:"features,omitempty"`
	Pieces    []AppliedPiece    `json:"pieces,omitempty"`
}

// AppliedPiece is one piece already adopted, with its own drift anchor —
// a common piece tracks its own repository, not the form's (ADR-0016).
type AppliedPiece struct {
	Name   string `json:"name" yaml:"name"`
	Source string `json:"source,omitempty" yaml:"source"`
	Commit string `json:"commit,omitempty" yaml:"commit"`
}

// maxScanDepth bounds the recursive walk of the repositories folder.
const maxScanDepth = 4

// ScanProjects walks the repositories folder recursively looking for
// Templetry projects (.templetry-answers.yml) and plain git repositories.
// A found project or repo is a leaf — its subtree is never descended into.
func (a *App) ScanProjects() ([]LocalProject, error) {
	parent := effectiveParentDir()
	if parent == "" {
		return nil, fmt.Errorf("set your repositories folder in Settings first")
	}
	if _, err := os.Stat(parent); err != nil {
		return nil, fmt.Errorf("repositories folder %s does not exist — update it in Settings", parent)
	}
	out := []LocalProject{}
	scanDir(parent, parent, 0, &out)
	return out, nil
}

func scanDir(root, dir string, depth int, out *[]LocalProject) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "node_modules" {
			continue
		}
		sub := filepath.Join(dir, e.Name())
		rel, _ := filepath.Rel(root, sub)
		if p, ok := readAnswers(sub); ok {
			p.Name = e.Name()
			p.Rel = filepath.ToSlash(rel)
			p.Remote = gitRemote(sub)
			p.Branch = gitBranch(sub)
			p.Host, p.Owner = ownerFromRemote(p.Remote)
			p.AvatarURL = avatarFor(p.Host, p.Owner)
			*out = append(*out, p)
			continue
		}
		if _, err := os.Stat(filepath.Join(sub, ".git")); err == nil {
			remote := gitRemote(sub)
			host, owner := ownerFromRemote(remote)
			*out = append(*out, LocalProject{
				Dir: sub, Name: e.Name(), Rel: filepath.ToSlash(rel),
				Kind: "git", Remote: remote, Branch: gitBranch(sub),
				Host: host, Owner: owner, AvatarURL: avatarFor(host, owner),
			})
			continue
		}
		if depth+1 < maxScanDepth {
			scanDir(root, sub, depth+1, out)
		}
	}
}

// readAnswers loads a project's provenance record, if the directory has one.
func readAnswers(dir string) (LocalProject, bool) {
	data, err := os.ReadFile(filepath.Join(dir, ".templetry-answers.yml"))
	if err != nil {
		return LocalProject{}, false
	}
	var ans struct {
		Template struct {
			Name   string `yaml:"name"`
			Source string `yaml:"source"`
			Commit string `yaml:"commit"`
		} `yaml:"template"`
		Variables map[string]string `yaml:"variables"`
		Features  map[string]bool   `yaml:"features"`
		Pieces    []AppliedPiece    `yaml:"pieces"`
	}
	if err := yaml.Unmarshal(data, &ans); err != nil {
		return LocalProject{}, false
	}
	return LocalProject{
		Dir: dir, Kind: "templetry",
		Template: ans.Template.Name, Source: ans.Template.Source, Commit: ans.Template.Commit,
		Variables: ans.Variables, Features: ans.Features, Pieces: ans.Pieces,
	}, true
}

// ownerFromRemote pulls the host and the account out of an origin URL.
// Both forms have to work: https://host/owner/name.git and the ssh
// git@host:owner/name.git, which is not a URL at all.
//
// The owner keeps every segment above the repository — a GitLab project can
// live several groups deep, and flattening that would merge unrelated ones.
func ownerFromRemote(remote string) (host, owner string) {
	r := strings.TrimSpace(remote)
	if r == "" {
		return "", ""
	}
	r = strings.TrimSuffix(r, ".git")

	switch {
	case strings.HasPrefix(r, "git@"):
		// git@host:owner/name
		rest := strings.TrimPrefix(r, "git@")
		h, path, ok := strings.Cut(rest, ":")
		if !ok {
			return "", ""
		}
		host, r = h, path
	case strings.Contains(r, "://"):
		_, rest, _ := strings.Cut(r, "://")
		// Drop any user@ before the host.
		if i := strings.Index(rest, "@"); i >= 0 && i < strings.Index(rest+"/", "/") {
			rest = rest[i+1:]
		}
		h, path, ok := strings.Cut(rest, "/")
		if !ok {
			return "", ""
		}
		// A port is not part of the host we build an avatar URL from.
		host, r = h, path
	default:
		return "", ""
	}

	i := strings.LastIndex(r, "/")
	if i <= 0 {
		return host, ""
	}
	return host, r[:i]
}

// avatarFor guesses the account's avatar. GitHub, Gitea and Forgejo all
// serve one at /<account>.png; where that guess is wrong the image simply
// fails to load and the UI falls back to a placeholder, which is the honest
// outcome for an account we cannot see.
func avatarFor(host, owner string) string {
	if host == "" || owner == "" {
		return ""
	}
	top := owner
	if i := strings.Index(owner, "/"); i >= 0 {
		top = owner[:i]
	}
	return "https://" + host + "/" + top + ".png"
}

// gitRemote reads origin's URL straight from .git/config — no git spawn.
func gitRemote(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, ".git", "config"))
	if err != nil {
		return ""
	}
	inOrigin := false
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") {
			inOrigin = t == `[remote "origin"]`
			continue
		}
		if inOrigin {
			if k, v, ok := strings.Cut(t, "="); ok && strings.TrimSpace(k) == "url" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

// gitBranch reads .git/HEAD directly — branch name, or a short sha when detached.
func gitBranch(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(data))
	if name, ok := strings.CutPrefix(head, "ref: refs/heads/"); ok {
		return name
	}
	if len(head) > 7 {
		return head[:7]
	}
	return head
}

// Drift marks a project with something to pull: its template moved past the
// recorded commit, or an applied piece did.
type Drift struct {
	Dir string `json:"dir"`
	// Latest is the template's current head; empty when only pieces moved.
	Latest string `json:"latest,omitempty"`
	// Pieces names the applied pieces whose own source moved.
	Pieces []string `json:"pieces,omitempty"`
}

// CheckDrift compares each project's recorded commits against their current
// heads — the template's and every applied piece's, since a common piece
// tracks its own repository (ADR-0016). Works on any forge the source
// scheme names (ADR-0015). One API call per distinct source@ref.
func (a *App) CheckDrift() ([]Drift, error) {
	projects, err := a.ScanProjects()
	if err != nil {
		return nil, err
	}
	cache := map[string]string{}
	// head resolves a recorded source string to its current commit, or ""
	// when it cannot be reached — an unreachable source is not drift.
	head := func(recorded string) string {
		src, gitRef, _, err := source.ParseSourceString(recorded)
		if err != nil {
			return ""
		}
		key := src.String() + "@" + gitRef
		if sha, seen := cache[key]; seen {
			return sha
		}
		sha, err := source.ResolveRef(src, gitRef, a.templateToken(src))
		if err != nil {
			sha = ""
		}
		cache[key] = sha
		return sha
	}

	out := []Drift{}
	for _, p := range projects {
		if p.Kind != "templetry" {
			continue
		}
		d := Drift{Dir: p.Dir}
		if p.Commit != "" {
			if latest := head(p.Source); latest != "" && latest != p.Commit {
				d.Latest = latest
			}
		}
		for _, pc := range p.Pieces {
			if pc.Commit == "" || pc.Source == "" {
				continue
			}
			if latest := head(pc.Source); latest != "" && latest != pc.Commit {
				d.Pieces = append(d.Pieces, pc.Name)
			}
		}
		if d.Latest != "" || len(d.Pieces) > 0 {
			out = append(out, d)
		}
	}
	return out, nil
}

// LocalRemote is one configured git remote.
type LocalRemote struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// LocalOverview is the Local preview: state summary of one repo on disk.
type LocalOverview struct {
	Branch     string        `json:"branch"`
	Branches   []string      `json:"branches"`
	Remotes    []LocalRemote `json:"remotes"`
	LastCommit string        `json:"lastCommit"`
	Changes    int           `json:"changes"`
	Docs       []string      `json:"docs"`
}

// gitOut runs a read-only git command in dir; ok=false when git fails.
func gitOut(dir string, args ...string) (string, bool) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// GetLocalOverview assembles the Local preview for one repo directory.
// Git failures degrade to partial data, never to an error.
func (a *App) GetLocalOverview(dir string) (LocalOverview, error) {
	out := LocalOverview{Branch: gitBranch(dir), Changes: -1}
	if s, ok := gitOut(dir, "branch", "--format=%(refname:short)"); ok && s != "" {
		out.Branches = strings.Split(s, "\n")
	}
	if s, ok := gitOut(dir, "remote"); ok && s != "" {
		for _, name := range strings.Split(s, "\n") {
			if u, ok := gitOut(dir, "remote", "get-url", name); ok {
				out.Remotes = append(out.Remotes, LocalRemote{name, u})
			}
		}
	}
	if s, ok := gitOut(dir, "log", "-1", "--format=%h · %s · %cs"); ok {
		out.LastCommit = s
	}
	if s, ok := gitOut(dir, "status", "--porcelain"); ok {
		if s == "" {
			out.Changes = 0
		} else {
			out.Changes = len(strings.Split(s, "\n"))
		}
	}
	for _, sub := range []string{"", "docs"} {
		entries, err := os.ReadDir(filepath.Join(dir, sub))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") || len(out.Docs) >= 40 {
				continue
			}
			out.Docs = append(out.Docs, filepath.ToSlash(filepath.Join(sub, e.Name())))
		}
	}
	sort.SliceStable(out.Docs, func(i, j int) bool {
		ri := strings.EqualFold(out.Docs[i], "README.md")
		rj := strings.EqualFold(out.Docs[j], "README.md")
		if ri != rj {
			return ri
		}
		return out.Docs[i] < out.Docs[j]
	})
	return out, nil
}

// GetLocalDoc reads one markdown file inside a scanned repo directory.
func (a *App) GetLocalDoc(dir, rel string) (string, error) {
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid path")
	}
	data, err := os.ReadFile(filepath.Join(dir, clean))
	if err != nil {
		return "", err
	}
	if len(data) > 512*1024 {
		data = data[:512*1024]
	}
	return string(data), nil
}

// PieceOption is one piece a project can adopt.
type PieceOption struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Applied     bool   `json:"applied"`
	// Common marks a piece that lives in a shared repository rather than in
	// the project's own form (ADR-0016).
	Common    bool                `json:"common,omitempty"`
	Variables []manifest.Variable `json:"variables,omitempty"`
}

// pieceRegistry is what the piece resolver consults for common pieces: the
// `pieces` arrays of every loaded catalog, merged, official first so its
// implementations win a tie. Catalogs load lazily, because the Local view
// can be the first thing a user opens.
func (a *App) pieceRegistry() *catalog.Registry {
	a.mu.Lock()
	empty := len(a.regs) == 0
	a.mu.Unlock()
	if empty {
		a.GetCatalogs() // per-catalog failures are reported there, not here
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	names := make([]string, 0, len(a.regs))
	for name := range a.regs {
		if name != OfficialCatalogName {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if a.regs[OfficialCatalogName] != nil {
		names = append([]string{OfficialCatalogName}, names...)
	}

	merged := &catalog.Registry{}
	seen := map[string]bool{}
	for _, name := range names {
		for _, p := range a.regs[name].Pieces {
			// Several entries legitimately share a name with disjoint
			// applies_to, so identity is name + where it comes from.
			k := p.Name + "|" + p.Repo + "|" + p.Path
			if seen[k] {
				continue
			}
			seen[k] = true
			merged.Pieces = append(merged.Pieces, p)
		}
	}
	return merged
}

// formManifestOf reads the form's manifest out of a fetched form FileSet.
func formManifestOf(files *source.FileSet) (*manifest.Manifest, error) {
	mf := files.Get("template.yml")
	if mf == nil {
		mf = files.Get("template.yaml")
	}
	if mf == nil {
		return nil, fmt.Errorf("the template no longer has a template.yml")
	}
	return manifest.Load(mf.Data)
}

// ListPieces returns everything the project can adopt — the pieces its form
// ships plus the registry's common pieces that support its template — with
// each piece's own variables so the UI can ask for them (ADR-0014, ADR-0016).
func (a *App) ListPieces(dir string) ([]PieceOption, error) {
	ans, err := answers.Read(dir)
	if err != nil {
		return nil, err
	}
	files, _, err := piece.FetchForm(ans)
	if err != nil {
		return nil, err
	}
	reg := a.pieceRegistry()
	out := []PieceOption{}
	for _, info := range piece.Available(files, reg, ans) {
		opt := PieceOption{
			Name: info.Name, Description: info.Description,
			Applied: info.Applied, Common: info.Common,
		}
		if res, err := piece.Resolve(info.Name, files, ans.Template.Source, reg, ans); err == nil {
			opt.Variables = res.Manifest.Variables
		}
		out = append(out, opt)
	}
	return out, nil
}

// AddPiece adopts one piece into an existing project and records it.
func (a *App) AddPiece(dir, name string, vars map[string]string) (string, error) {
	ans, err := answers.Read(dir)
	if err != nil {
		return "", err
	}
	for _, p := range ans.Pieces {
		if p.Name == name {
			return "", fmt.Errorf("piece %q is already applied", name)
		}
	}
	files, commit, err := piece.FetchForm(ans)
	if err != nil {
		return "", err
	}
	formM, err := formManifestOf(files)
	if err != nil {
		return "", err
	}
	resolved, err := piece.Resolve(name, files, ans.Template.Source, a.pieceRegistry(), ans)
	if err != nil {
		return "", err
	}
	if resolved.Common {
		// A common piece has its own head, unrelated to the form's.
		commit = resolved.Commit
	}
	res, err := piece.Apply(dir, formM, resolved.Manifest, resolved.Files, ans.Variables, vars)
	if err != nil {
		return "", err
	}

	// A common piece records its own repository, so updates follow it there
	// rather than looking inside the form (ADR-0016).
	src := resolved.Source
	if !resolved.Common {
		src = ans.Template.Source
		if src != "" && src != "local" {
			src += "/pieces/" + name
		}
	}
	ans.Pieces = append(ans.Pieces, answers.AppliedPiece{
		Name: name, Source: src, Commit: commit,
		Variables: res.Variables, Files: res.Files,
	})
	if err := answers.Write(dir, ans); err != nil {
		return "", err
	}
	msg := fmt.Sprintf("%s applied: %d files", name, len(res.Files))
	if len(resolved.Manifest.Patches) > 0 {
		msg += fmt.Sprintf(" + %d patches", len(resolved.Manifest.Patches))
	}
	return msg + " — review with git before committing", nil
}

// OpenFolder shows a project directory in the system file explorer.
func (a *App) OpenFolder(dir string) {
	_ = exec.Command("explorer", dir).Start()
}
