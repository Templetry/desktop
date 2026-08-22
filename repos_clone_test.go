package main

import (
	"path/filepath"
	"testing"
)

func TestCloneTarget(t *testing.T) {
	parent := filepath.Join("C:", "repos")
	cases := []struct {
		name, full, layout, wantDir, wantSub string
	}{
		{"owner layout nests under the organization",
			"Templetry/engine", "owner", filepath.Join(parent, "Templetry", "engine"), "Templetry/engine"},
		{"flat layout keeps the old shape",
			"Templetry/engine", "flat", filepath.Join(parent, "engine"), "engine"},
		// GitLab groups nest, and the whole path is the owner.
		{"a nested group stays nested",
			"group/sub/proj", "owner", filepath.Join(parent, "group", "sub", "proj"), "group/sub/proj"},
		// Nothing to group by: do not invent a folder.
		{"a bare name falls back to flat",
			"engine", "owner", filepath.Join(parent, "engine"), "engine"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, sub := cloneTarget(parent, c.full, c.layout)
			if dir != c.wantDir {
				t.Errorf("dir = %q, want %q", dir, c.wantDir)
			}
			if sub != c.wantSub {
				t.Errorf("sub = %q, want %q", sub, c.wantSub)
			}
		})
	}
}

// The default has to be the grouped one: an empty setting is what every
// existing installation has in its config file.
func TestCloneLayoutDefaultsToOwner(t *testing.T) {
	if got := withDefaults(appConfig{}).CloneLayout; got != "owner" {
		t.Errorf("default layout = %q, want %q", got, "owner")
	}
}
