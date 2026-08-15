package main

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// The Cloud preview for forges other than GitHub (ADR-0015). The shape is
// the same RepoOverview the GitHub path builds, so the UI does not care
// where a repository lives; only the endpoints differ.
//
// Every sub-request degrades independently: a forge without a CI API, or a
// project whose tree is not readable, still yields a useful overview.

// glProject is a GitLab project path, URL-encoded the way the API wants it.
func glProject(fullName string) string { return url.PathEscape(fullName) }

// forgeTreeEntry is one entry of a repository tree, in the shape both
// GitLab and Gitea report it.
type forgeTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

// forgeTreeLimit bounds how much of a repository tree is read. Both forges
// truncate anyway — Gitea returns 1000 of a 10,000-entry tree and says so
// only in a field nobody reads — so the cap is stated here rather than
// inherited by accident. It is ample for what the tree is used for: finding
// template manifests, which live near the root, and the first 40 markdown
// files.
const forgeTreeLimit = 1000

// gitlabTreePage is GitLab's maximum page size.
const gitlabTreePage = 100

// gitlabTree reads a project's recursive tree.
//
// GitLab caps a page at 100 entries and walks depth-first, so one page of a
// large repository can be nothing but directories — which is exactly how the
// docs list came back empty for a project full of markdown. Pages are read
// until one comes back short.
func gitlabTree(acc Account, token, fullName string) ([]forgeTreeEntry, error) {
	base := forgeAPI(acc.Scheme, acc.Host)
	out := []forgeTreeEntry{}
	for page := 1; page <= forgeTreeLimit/gitlabTreePage; page++ {
		var batch []forgeTreeEntry
		u := fmt.Sprintf("%s/projects/%s/repository/tree?recursive=true&per_page=%d&page=%d",
			base, glProject(fullName), gitlabTreePage, page)
		if err := forgeDo("GET", u, acc.Scheme, token, nil, &batch); err != nil {
			if page == 1 {
				return nil, err
			}
			break // a partial tree beats none
		}
		out = append(out, batch...)
		if len(batch) < gitlabTreePage {
			break
		}
	}
	return out, nil
}

// giteaTree reads a repository tree in one request. Gitea truncates at its
// own limit and reports it in `truncated`, which the caller cannot act on —
// so the cap is the same as GitLab's walk, and both are documented rather
// than surprising.
func giteaTree(acc Account, token, fullName, branch string) ([]forgeTreeEntry, error) {
	var tree struct {
		Tree      []forgeTreeEntry `json:"tree"`
		Truncated bool             `json:"truncated"`
	}
	u := fmt.Sprintf("%s/repos/%s/git/trees/%s?recursive=1&per_page=%d",
		forgeAPI(acc.Scheme, acc.Host), fullName, url.PathEscape(branch), forgeTreeLimit)
	if err := forgeDo("GET", u, acc.Scheme, token, nil, &tree); err != nil {
		return nil, err
	}
	return tree.Tree, nil
}

// forgeRepoOverview assembles the Cloud preview for a GitLab or Gitea repo.
func forgeRepoOverview(acc Account, token, fullName string) (RepoOverview, error) {
	switch acc.Scheme {
	case "gitlab":
		return gitlabOverview(acc, token, fullName)
	case "gitea":
		return giteaOverview(acc, token, fullName)
	}
	return RepoOverview{}, fmt.Errorf("no preview for %s repositories", acc.Scheme)
}

// forgeRepoDoc fetches one markdown file from a GitLab or Gitea repo.
func forgeRepoDoc(acc Account, token, fullName, docPath string) (string, error) {
	base := forgeAPI(acc.Scheme, acc.Host)
	switch acc.Scheme {
	case "gitlab":
		// GitLab wants the file path escaped as a single path segment, and
		// a ref is mandatory — so the default branch has to be resolved.
		var meta struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := forgeDo("GET", base+"/projects/"+glProject(fullName), acc.Scheme, token, nil, &meta); err != nil {
			return "", err
		}
		u := base + "/projects/" + glProject(fullName) +
			"/repository/files/" + url.PathEscape(docPath) +
			"?ref=" + url.QueryEscape(meta.DefaultBranch)
		var res struct {
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		}
		if err := forgeDo("GET", u, acc.Scheme, token, nil, &res); err != nil {
			return "", err
		}
		return decodeContent(res.Content, res.Encoding)
	case "gitea":
		var res struct {
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		}
		u := base + "/repos/" + fullName + "/contents/" + docPath
		if err := forgeDo("GET", u, acc.Scheme, token, nil, &res); err != nil {
			return "", err
		}
		return decodeContent(res.Content, res.Encoding)
	}
	return "", fmt.Errorf("no document reader for %s", acc.Scheme)
}

func gitlabOverview(acc Account, token, fullName string) (RepoOverview, error) {
	base := forgeAPI(acc.Scheme, acc.Host)
	proj := base + "/projects/" + glProject(fullName)
	out := RepoOverview{}

	var meta struct {
		Description   string `json:"description"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := forgeDo("GET", proj, acc.Scheme, token, nil, &meta); err != nil {
		return out, err
	}
	out.Description, out.DefaultBranch = meta.Description, meta.DefaultBranch

	// GitLab reports language shares as percentages already.
	langs := map[string]float64{}
	if err := forgeDo("GET", proj+"/languages", acc.Scheme, token, nil, &langs); err == nil {
		for k, v := range langs {
			if int(v) >= 1 {
				out.Languages = append(out.Languages, LangShare{k, int(v)})
			}
		}
		sort.Slice(out.Languages, func(i, j int) bool { return out.Languages[i].Pct > out.Languages[j].Pct })
	}

	var branches []struct {
		Name string `json:"name"`
	}
	if err := forgeDo("GET", proj+"/repository/branches?per_page=100", acc.Scheme, token, nil, &branches); err == nil {
		for _, b := range branches {
			out.Branches = append(out.Branches, b.Name)
		}
	}

	var pipelines []struct {
		Ref       string `json:"ref"`
		Status    string `json:"status"`
		WebURL    string `json:"web_url"`
		UpdatedAt string `json:"updated_at"`
		Name      string `json:"name"`
	}
	if err := forgeDo("GET", proj+"/pipelines?per_page=6", acc.Scheme, token, nil, &pipelines); err == nil {
		for _, p := range pipelines {
			name := p.Name
			if name == "" {
				name = "pipeline"
			}
			status, conclusion := ciStatus(p.Status)
			out.Runs = append(out.Runs, CIRun{name, p.Ref, status, conclusion, p.WebURL, p.UpdatedAt})
		}
	}

	if tree, err := gitlabTree(acc, token, fullName); err == nil {
		for _, t := range tree {
			if t.Type != "blob" {
				continue
			}
			collectTreeEntry(&out, t.Path)
		}
		sortDocs(out.Docs)
	}
	return out, nil
}

func giteaOverview(acc Account, token, fullName string) (RepoOverview, error) {
	base := forgeAPI(acc.Scheme, acc.Host)
	repo := base + "/repos/" + fullName
	out := RepoOverview{}

	var meta struct {
		Description   string `json:"description"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := forgeDo("GET", repo, acc.Scheme, token, nil, &meta); err != nil {
		return out, err
	}
	out.Description, out.DefaultBranch = meta.Description, meta.DefaultBranch

	// Gitea reports bytes per language, like GitHub.
	langs := map[string]int64{}
	if err := forgeDo("GET", repo+"/languages", acc.Scheme, token, nil, &langs); err == nil {
		var total int64
		for _, v := range langs {
			total += v
		}
		for k, v := range langs {
			if total > 0 && int(v*100/total) >= 1 {
				out.Languages = append(out.Languages, LangShare{k, int(v * 100 / total)})
			}
		}
		sort.Slice(out.Languages, func(i, j int) bool { return out.Languages[i].Pct > out.Languages[j].Pct })
	}

	var branches []struct {
		Name string `json:"name"`
	}
	if err := forgeDo("GET", repo+"/branches?limit=100", acc.Scheme, token, nil, &branches); err == nil {
		for _, b := range branches {
			out.Branches = append(out.Branches, b.Name)
		}
	}

	// Gitea Actions is recent and optional; older or disabled instances
	// 404 here, which simply leaves the CI strip empty.
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
	if err := forgeDo("GET", repo+"/actions/runs?limit=6", acc.Scheme, token, nil, &runs); err == nil {
		for _, r := range runs.WorkflowRuns {
			out.Runs = append(out.Runs, CIRun{r.Name, r.HeadBranch, r.Status, r.Conclusion, r.HTMLURL, r.UpdatedAt})
		}
	}

	if meta.DefaultBranch != "" {
		if tree, err := giteaTree(acc, token, fullName, meta.DefaultBranch); err == nil {
			for _, t := range tree {
				if t.Type != "blob" {
					continue
				}
				collectTreeEntry(&out, t.Path)
			}
			sortDocs(out.Docs)
		}
	}
	return out, nil
}

// ciStatus maps a GitLab pipeline status onto the (status, conclusion) pair
// the UI already speaks, which is GitHub's shape.
func ciStatus(s string) (status, conclusion string) {
	switch s {
	case "success":
		return "completed", "success"
	case "failed":
		return "completed", "failure"
	case "canceled", "skipped":
		return "completed", s
	case "":
		return "", ""
	default: // created, pending, running, manual, scheduled…
		return s, ""
	}
}

// forgeHasTemplate reports whether a repository carries a template.yml the
// engine could render, anywhere in its tree.
func forgeHasTemplate(acc Account, token, fullName, defaultBranch string) bool {
	switch acc.Scheme {
	case "gitlab":
		tree, err := gitlabTree(acc, token, fullName)
		if err != nil {
			return false
		}
		for _, t := range tree {
			if t.Type == "blob" && isTemplateManifest(t.Path) {
				return true
			}
		}
	case "gitea":
		if defaultBranch == "" {
			return false
		}
		tree, err := giteaTree(acc, token, fullName, defaultBranch)
		if err != nil {
			return false
		}
		for _, t := range tree {
			if t.Type == "blob" && isTemplateManifest(t.Path) {
				return true
			}
		}
	}
	return false
}

func isTemplateManifest(p string) bool {
	base := strings.ToLower(p[strings.LastIndex(p, "/")+1:])
	return base == "template.yml" || base == "template.yaml"
}
