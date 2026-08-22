package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Templetry/engine/manifest"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Repo is the slim listing the projects view renders.
type Repo struct {
	Name        string `json:"name"`
	FullName    string `json:"fullName"`
	Owner       string `json:"owner"`
	Description string `json:"description"`
	HTMLURL     string `json:"htmlUrl"`
	CloneURL    string `json:"cloneUrl"`
	Private     bool   `json:"private"`
	Language    string `json:"language"`
	UpdatedAt   string `json:"updatedAt"`
	Archived    bool   `json:"archived"`
	AvatarURL   string `json:"avatarUrl"`
	// Forge is the account key ("<scheme>@<host>") the repo came from;
	// empty means the GitHub OAuth session.
	Forge string `json:"forge,omitempty"`
	// DefaultBranch is needed to read a tree on forges whose tree endpoint
	// demands a ref. Not every listing supplies it.
	DefaultBranch string `json:"defaultBranch,omitempty"`
}

// repoKey identifies a repository across forges: the same owner/name can
// exist on GitHub and on a company GitLab, and they are not the same repo.
func repoKey(forge, fullName string) string {
	return strings.ToLower(forge + "::" + fullName)
}

// ListRepos returns the user's repositories across personal account and orgs,
// most recently updated first.
func (a *App) ListRepos() ([]Repo, error) {
	a.mu.Lock()
	token := a.token
	a.mu.Unlock()
	if token == "" && len(a.GetAccounts()) == 0 {
		return nil, fmt.Errorf("sign in first")
	}
	var out []Repo
	for page := 1; token != "" && page <= 3; page++ {
		url := fmt.Sprintf("https://api.github.com/user/repos?affiliation=owner,organization_member&sort=updated&per_page=100&page=%d", page)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		var batch []struct {
			Name        string `json:"name"`
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			HTMLURL     string `json:"html_url"`
			CloneURL    string `json:"clone_url"`
			Private     bool   `json:"private"`
			Language    string `json:"language"`
			UpdatedAt   string `json:"updated_at"`
			Archived    bool   `json:"archived"`
			Owner       struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
			} `json:"owner"`
		}
		err = json.NewDecoder(resp.Body).Decode(&batch)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		for _, r := range batch {
			out = append(out, Repo{
				Name: r.Name, FullName: r.FullName, Owner: r.Owner.Login,
				Description: r.Description, HTMLURL: r.HTMLURL, CloneURL: r.CloneURL,
				Private: r.Private, Language: r.Language, UpdatedAt: r.UpdatedAt,
				Archived: r.Archived, AvatarURL: r.Owner.AvatarURL,
			})
		}
		if len(batch) < 100 {
			break
		}
	}
	// Merge in every other signed-in forge account (ADR-0015).
	for _, acc := range a.GetAccounts() {
		if acc.Scheme == "github" {
			continue
		}
		tok, err := accountToken(acc)
		if err != nil {
			continue
		}
		more, err := forgeListRepos(acc, tok)
		if err != nil {
			continue // one bad account must not blank the whole list
		}
		out = append(out, more...)
	}
	return out, nil
}

// ghJSON performs an authenticated (when signed in) GitHub API GET.
func (a *App) ghJSON(u string, v any) error {
	a.mu.Lock()
	token := a.token
	a.mu.Unlock()
	req, _ := http.NewRequest("GET", u, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API: HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// ListTemplateRepos returns owner/name (lowercased) of every repo whose
// default branch carries a template.yml — repos the engine can render.
// One code-search call covers the user plus all their orgs.
func (a *App) ListTemplateRepos() ([]string, error) {
	a.mu.Lock()
	token, login := a.token, a.auth.Login
	a.mu.Unlock()

	seen := map[string]bool{}
	out := []string{}
	add := func(forge, fullName string) {
		k := repoKey(forge, fullName)
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}

	// GitHub answers in one code-search call covering the user and orgs.
	if token != "" {
		q := "filename:template.yml user:" + login
		var orgs []struct {
			Login string `json:"login"`
		}
		if err := a.ghJSON("https://api.github.com/user/orgs", &orgs); err == nil {
			for _, o := range orgs {
				q += " org:" + o.Login
			}
		}
		var res struct {
			Items []struct {
				Repository struct {
					FullName string `json:"full_name"`
				} `json:"repository"`
			} `json:"items"`
		}
		if err := a.ghJSON("https://api.github.com/search/code?per_page=100&q="+url.QueryEscape(q), &res); err == nil {
			for _, it := range res.Items {
				add("", it.Repository.FullName)
			}
		}
	}

	// Other forges have no comparable content search across repositories,
	// so each candidate's tree is read instead — bounded, because that is
	// one request per repository.
	for _, acc := range a.GetAccounts() {
		if acc.Scheme == "github" {
			continue
		}
		tok, err := accountToken(acc)
		if err != nil {
			continue
		}
		repos, err := forgeListRepos(acc, tok)
		if err != nil {
			continue
		}
		if len(repos) > templateProbeLimit {
			repos = repos[:templateProbeLimit]
		}
		for _, r := range probeTemplates(acc, tok, repos) {
			add(acc.Key(), r)
		}
	}

	if len(out) == 0 && token == "" && len(a.GetAccounts()) == 0 {
		return nil, fmt.Errorf("sign in first")
	}
	return out, nil
}

// templateProbeLimit caps how many repositories per account get a tree read
// when looking for templates. Beyond it, repos simply go unflagged rather
// than the listing turning into a request storm.
const templateProbeLimit = 40

// probeTemplates reads repository trees concurrently and returns the full
// names carrying a template.yml.
func probeTemplates(acc Account, token string, repos []Repo) []string {
	type result struct {
		name string
		ok   bool
	}
	jobs := make(chan Repo)
	results := make(chan result)
	workers := 6
	if len(repos) < workers {
		workers = len(repos)
	}
	if workers == 0 {
		return nil
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range jobs {
				results <- result{r.FullName, forgeHasTemplate(acc, token, r.FullName, r.DefaultBranch)}
			}
		}()
	}
	go func() {
		for _, r := range repos {
			jobs <- r
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	out := []string{}
	for res := range results {
		if res.ok {
			out = append(out, res.name)
		}
	}
	return out
}

// LangShare is one language's slice of a repo, in percent.
type LangShare struct {
	Name string `json:"name"`
	Pct  int    `json:"pct"`
}

// CIRun is one workflow run in the repo overview.
type CIRun struct {
	Name       string `json:"name"`
	Branch     string `json:"branch"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	URL        string `json:"url"`
	UpdatedAt  string `json:"updatedAt"`
}

// TemplateForm is a form found inside a repository: something the engine
// could render, described by its own manifest (ADR-0017), so a template repo
// says what it is instead of only that it is one.
type TemplateForm struct {
	Path        string   `json:"path"` // directory holding the manifest; "." for the root
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Kinds       []string `json:"kinds,omitempty"`
	Languages   []string `json:"languages,omitempty"`
	Frameworks  []string `json:"frameworks,omitempty"`
}

// RepoOverview is the Cloud preview: state summary of one repository, on
// whichever forge it lives.
type RepoOverview struct {
	Description   string         `json:"description"`
	DefaultBranch string         `json:"defaultBranch"`
	Languages     []LangShare    `json:"languages"`
	Branches      []string       `json:"branches"`
	Runs          []CIRun        `json:"runs"`
	Docs          []string       `json:"docs"`
	TemplateForms []TemplateForm `json:"templateForms"`

	// manifests are the manifest paths the tree walk found, kept between
	// building the overview and describing the forms. Unexported, so it
	// never reaches the UI.
	manifests []string
}

// repoAccount resolves the account a Cloud row belongs to, with its token.
// An empty forge key means the GitHub OAuth session.
func (a *App) repoAccount(forge string) (Account, string, error) {
	if forge == "" {
		a.mu.Lock()
		token, auth := a.token, a.auth
		a.mu.Unlock()
		if token == "" {
			return Account{}, "", fmt.Errorf("sign in first")
		}
		return Account{Scheme: "github", Host: "github.com", Login: auth.Login}, token, nil
	}
	for _, acc := range a.GetAccounts() {
		if acc.Key() != forge {
			continue
		}
		if acc.Scheme == "github" {
			a.mu.Lock()
			token := a.token
			a.mu.Unlock()
			return acc, token, nil
		}
		token, err := accountToken(acc)
		if err != nil {
			return Account{}, "", fmt.Errorf("no stored token for %s — sign in again", forge)
		}
		return acc, token, nil
	}
	return Account{}, "", fmt.Errorf("no signed-in account for %s", forge)
}

// GetRepoOverview assembles the Cloud preview for one repository. The forge
// key comes from the listing row; empty means the GitHub OAuth session.
func (a *App) GetRepoOverview(fullName, forge string) (RepoOverview, error) {
	var out RepoOverview
	var err error
	if forge != "" && !strings.HasPrefix(forge, "github@") {
		acc, token, aerr := a.repoAccount(forge)
		if aerr != nil {
			return RepoOverview{}, aerr
		}
		out, err = forgeRepoOverview(acc, token, fullName)
	} else {
		out, err = a.githubRepoOverview(fullName)
	}
	if err != nil {
		return out, err
	}
	// Reading the manifests is a separate pass so both forge paths get it
	// from one place — and so a template repo can show what it *is*, not
	// merely that it is one.
	out.TemplateForms = a.describeForms(fullName, forge, out.manifests)
	return out, nil
}

// maxDescribedForms bounds the manifest reads: one request each, and a repo
// with more forms than this is describing itself well enough already.
const maxDescribedForms = 12

// describeForms reads each manifest and reports its taxonomy. A manifest
// that cannot be fetched or does not parse still yields its path — the file
// is there, it simply says nothing the engine understands.
func (a *App) describeForms(fullName, forge string, manifests []string) []TemplateForm {
	if len(manifests) > maxDescribedForms {
		manifests = manifests[:maxDescribedForms]
	}
	out := make([]TemplateForm, len(manifests))
	var wg sync.WaitGroup
	for i, p := range manifests {
		out[i] = TemplateForm{Path: path.Dir(p)}
		wg.Add(1)
		go func(i int, p string) {
			defer wg.Done()
			data, err := a.GetRepoDoc(fullName, p, forge)
			if err != nil {
				return
			}
			m, err := manifest.Load([]byte(data))
			if err != nil {
				return
			}
			out[i].Name = m.Name
			out[i].Description = m.Description
			out[i].Kinds = m.Kinds
			out[i].Languages = m.Languages
			out[i].Frameworks = m.Frameworks
		}(i, p)
	}
	wg.Wait()
	return out
}

// githubRepoOverview is the GitHub implementation. Each sub-request degrades
// independently — a repo without Actions or docs still yields a useful
// overview.
func (a *App) githubRepoOverview(fullName string) (RepoOverview, error) {
	out := RepoOverview{}
	base := "https://api.github.com/repos/" + fullName
	var meta struct {
		Description   string `json:"description"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := a.ghJSON(base, &meta); err != nil {
		return out, err
	}
	out.Description, out.DefaultBranch = meta.Description, meta.DefaultBranch

	langs := map[string]int{}
	if err := a.ghJSON(base+"/languages", &langs); err == nil {
		total := 0
		for _, v := range langs {
			total += v
		}
		for k, v := range langs {
			if total > 0 && v*100/total >= 1 {
				out.Languages = append(out.Languages, LangShare{k, v * 100 / total})
			}
		}
		sort.Slice(out.Languages, func(i, j int) bool { return out.Languages[i].Pct > out.Languages[j].Pct })
	}

	var branches []struct {
		Name string `json:"name"`
	}
	if err := a.ghJSON(base+"/branches?per_page=100", &branches); err == nil {
		for _, b := range branches {
			out.Branches = append(out.Branches, b.Name)
		}
	}

	var runs struct {
		WorkflowRuns []struct {
			Name       string `json:"name"`
			HeadBranch string `json:"head_branch"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			HTMLURL    string `json:"html_url"`
			UpdatedAt  string `json:"updated_at"`
		} `json:"workflow_runs"`
	}
	if err := a.ghJSON(base+"/actions/runs?per_page=6", &runs); err == nil {
		for _, r := range runs.WorkflowRuns {
			out.Runs = append(out.Runs, CIRun{r.Name, r.HeadBranch, r.Status, r.Conclusion, r.HTMLURL, r.UpdatedAt})
		}
	}

	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := a.ghJSON(base+"/git/trees/"+meta.DefaultBranch+"?recursive=1", &tree); err == nil {
		for _, t := range tree.Tree {
			if t.Type != "blob" {
				continue
			}
			collectTreeEntry(&out, t.Path)
		}
		sortDocs(out.Docs)
	}
	return out, nil
}

// sortDocs orders a markdown listing: README first, then shallow before
// deep, then alphabetical.
func sortDocs(docs []string) {
	sort.SliceStable(docs, func(i, j int) bool {
		ri := strings.EqualFold(docs[i], "README.md")
		rj := strings.EqualFold(docs[j], "README.md")
		if ri != rj {
			return ri
		}
		di, dj := strings.Count(docs[i], "/"), strings.Count(docs[j], "/")
		if di != dj {
			return di < dj
		}
		return docs[i] < docs[j]
	})
}

// collectTreeEntry folds one repository tree blob into an overview: forms
// the engine could render, and markdown worth reading.
func collectTreeEntry(out *RepoOverview, p string) {
	name := strings.ToLower(path.Base(p))
	if name == "template.yml" || name == "template.yaml" {
		// The full path, not its directory: the manifest has to be fetched
		// afterwards to learn what the form is.
		out.manifests = append(out.manifests, p)
	}
	if strings.HasSuffix(name, ".md") && len(out.Docs) < 40 {
		out.Docs = append(out.Docs, p)
	}
}

// decodeContent unwraps a forge "contents" response, which is base64 on
// every forge that bothers to say so.
func decodeContent(content, encoding string) (string, error) {
	if encoding != "base64" {
		return content, nil
	}
	data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content, "\n", ""))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetRepoDoc fetches one markdown file of a repo, decoded.
func (a *App) GetRepoDoc(fullName, docPath, forge string) (string, error) {
	if forge != "" && !strings.HasPrefix(forge, "github@") {
		acc, token, err := a.repoAccount(forge)
		if err != nil {
			return "", err
		}
		return forgeRepoDoc(acc, token, fullName, docPath)
	}
	var res struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := a.ghJSON("https://api.github.com/repos/"+fullName+"/contents/"+docPath, &res); err != nil {
		return "", err
	}
	return decodeContent(res.Content, res.Encoding)
}

// OpenRepo opens a repository page in the default browser.
func (a *App) OpenRepo(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// cloneTarget decides where a clone lands. Under the "owner" layout the
// repository goes into a subfolder named after the account or organization
// it belongs to, which is the difference between a repositories folder you
// can read and one with fifty entries in alphabetical order. It also keeps
// two forges' "owner/name" collisions apart on disk.
func cloneTarget(parent, fullName, layout string) (dir, sub string) {
	owner, name := "", fullName
	if i := strings.LastIndex(fullName, "/"); i >= 0 {
		owner, name = fullName[:i], fullName[i+1:]
	}
	if layout == "flat" || owner == "" {
		return filepath.Join(parent, name), name
	}
	return filepath.Join(parent, owner, name), filepath.ToSlash(filepath.Join(owner, name))
}

// CloneRepo clones a repository into the remembered parent folder and
// returns the local path. It authenticates as the account the repository
// came from, so a private repo on any signed-in forge clones too.
func (a *App) CloneRepo(cloneURL, fullName, forge string) (string, error) {
	acc, token, err := a.repoAccount(forge)
	if err != nil {
		return "", err
	}
	parent := effectiveParentDir()
	if parent == "" {
		return "", fmt.Errorf("set your default repositories folder in Settings first")
	}
	target, sub := cloneTarget(parent, fullName, withDefaults(loadConfig()).CloneLayout)
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("%s already exists", target)
	}
	// git creates the leaf, but not the owner folder above it.
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if err := runGit(parent, gitAuthFor(acc, token), "clone", cloneURL, sub); err != nil {
		return "", err
	}
	return target, nil
}
