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
}

// ListRepos returns the user's repositories across personal account and orgs,
// most recently updated first.
func (a *App) ListRepos() ([]Repo, error) {
	a.mu.Lock()
	token := a.token
	a.mu.Unlock()
	if token == "" {
		return nil, fmt.Errorf("sign in first")
	}
	var out []Repo
	for page := 1; page <= 3; page++ {
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
	if token == "" {
		return nil, fmt.Errorf("sign in first")
	}
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
	if err := a.ghJSON("https://api.github.com/search/code?per_page=100&q="+url.QueryEscape(q), &res); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []string{}
	for _, it := range res.Items {
		fn := strings.ToLower(it.Repository.FullName)
		if !seen[fn] {
			seen[fn] = true
			out = append(out, fn)
		}
	}
	return out, nil
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

// RepoOverview is the Cloud preview: state summary of one GitHub repo.
type RepoOverview struct {
	Description   string      `json:"description"`
	DefaultBranch string      `json:"defaultBranch"`
	Languages     []LangShare `json:"languages"`
	Branches      []string    `json:"branches"`
	Runs          []CIRun     `json:"runs"`
	Docs          []string    `json:"docs"`
	TemplateForms []string    `json:"templateForms"`
}

// GetRepoOverview assembles the Cloud preview for one repository. Each
// sub-request degrades independently — a repo without Actions or docs
// still yields a useful overview.
func (a *App) GetRepoOverview(fullName string) (RepoOverview, error) {
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
			name := strings.ToLower(path.Base(t.Path))
			if name == "template.yml" || name == "template.yaml" {
				out.TemplateForms = append(out.TemplateForms, path.Dir(t.Path))
			}
			if strings.HasSuffix(name, ".md") && len(out.Docs) < 40 {
				out.Docs = append(out.Docs, t.Path)
			}
		}
		// README first, then shallow before deep, then alphabetical.
		sort.SliceStable(out.Docs, func(i, j int) bool {
			ri := strings.EqualFold(out.Docs[i], "README.md")
			rj := strings.EqualFold(out.Docs[j], "README.md")
			if ri != rj {
				return ri
			}
			di, dj := strings.Count(out.Docs[i], "/"), strings.Count(out.Docs[j], "/")
			if di != dj {
				return di < dj
			}
			return out.Docs[i] < out.Docs[j]
		})
	}
	return out, nil
}

// GetRepoDoc fetches one markdown file of a repo, decoded.
func (a *App) GetRepoDoc(fullName, docPath string) (string, error) {
	var res struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := a.ghJSON("https://api.github.com/repos/"+fullName+"/contents/"+docPath, &res); err != nil {
		return "", err
	}
	if res.Encoding == "base64" {
		data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(res.Content, "\n", ""))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return res.Content, nil
}

// OpenRepo opens a repository page in the default browser.
func (a *App) OpenRepo(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// CloneRepo clones a repository into the remembered parent folder and
// returns the local path.
func (a *App) CloneRepo(cloneURL, name string) (string, error) {
	a.mu.Lock()
	token, login := a.token, a.auth.Login
	a.mu.Unlock()
	if token == "" {
		return "", fmt.Errorf("sign in first")
	}
	parent := effectiveParentDir()
	if parent == "" {
		return "", fmt.Errorf("set your default repositories folder in Settings first")
	}
	target := filepath.Join(parent, name)
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("%s already exists", target)
	}
	if err := runGit(parent, token, login, "clone", cloneURL, name); err != nil {
		return "", err
	}
	return target, nil
}
