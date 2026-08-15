package main

import "testing"

// The tree walk records the manifest's full path, not its directory: the
// file has to be fetched afterwards to learn what the form is. Recording
// only the directory is what the Cloud panel used to do, and it is why a
// template repo could say it was one without saying what it was.
func TestCollectTreeEntryKeepsManifestPaths(t *testing.T) {
	var out RepoOverview
	for _, p := range []string{
		"README.md",
		"template.yml",
		"minimal-api/template.yml",
		"razor-web/template.yaml",
		"docs/guide.md",
		"src/main.go",
		"nested/TEMPLATE.YML",
	} {
		collectTreeEntry(&out, p)
	}

	want := []string{"template.yml", "minimal-api/template.yml", "razor-web/template.yaml", "nested/TEMPLATE.YML"}
	if len(out.manifests) != len(want) {
		t.Fatalf("manifests = %v, want %v", out.manifests, want)
	}
	for i := range want {
		if out.manifests[i] != want[i] {
			t.Errorf("manifests[%d] = %q, want %q", i, out.manifests[i], want[i])
		}
	}

	wantDocs := []string{"README.md", "docs/guide.md"}
	if len(out.Docs) != len(wantDocs) {
		t.Errorf("docs = %v, want %v", out.Docs, wantDocs)
	}
}

// The docs list is capped, but the cap must not stop the walk finding
// manifests — a form buried past the fortieth markdown file is still a form.
func TestManifestsSurviveTheDocsCap(t *testing.T) {
	var out RepoOverview
	for i := 0; i < 60; i++ {
		collectTreeEntry(&out, "docs/page.md")
	}
	collectTreeEntry(&out, "late/template.yml")

	if len(out.Docs) != 40 {
		t.Errorf("docs should cap at 40, got %d", len(out.Docs))
	}
	if len(out.manifests) != 1 || out.manifests[0] != "late/template.yml" {
		t.Errorf("the manifest should still be found, got %v", out.manifests)
	}
}
