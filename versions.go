package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	goruntime "runtime"
	"runtime/debug"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// version is stamped by the release build via -ldflags "-X main.version=...".
var version = "dev"

// VersionInfo is what the UI shows as the current build.
type VersionInfo struct {
	App    string `json:"app"`
	Engine string `json:"engine"`
}

// engineVersion reads the embedded engine module version from build info.
func engineVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path == "github.com/Templetry/engine" {
				return d.Version
			}
		}
	}
	return "unknown"
}

// GetVersions returns the running app and embedded engine versions.
// The app version is normalized without the tag's "v" prefix — the UI
// adds its own prefix when displaying it.
func (a *App) GetVersions() VersionInfo {
	return VersionInfo{App: norm(version), Engine: engineVersion()}
}

// UpdateInfo reports the latest published releases.
type UpdateInfo struct {
	AppLatest    string `json:"appLatest"`
	EngineLatest string `json:"engineLatest"`
	AppUpdate    bool   `json:"appUpdate"`
	EngineUpdate bool   `json:"engineUpdate"`
	AppURL       string `json:"appUrl"`
	EngineURL    string `json:"engineUrl"`
}

func latestRelease(repo string) (tag, url string, err error) {
	resp, err := http.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("%s: HTTP %d", repo, resp.StatusCode)
	}
	var r struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", "", err
	}
	return r.TagName, r.HTMLURL, nil
}

func norm(v string) string { return strings.TrimPrefix(strings.TrimSpace(v), "v") }

// InstallAppUpdate downloads the latest Windows installer from releases,
// verifies it against the release's SHA256SUMS, launches it and quits the
// app so the installer can replace it. Only a strictly newer release is
// accepted.
func (a *App) InstallAppUpdate() (string, error) {
	if goruntime.GOOS != "windows" {
		return "", fmt.Errorf("in-app update is Windows-only for now — download the new version from https://github.com/Templetry/desktop/releases")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	path, tag, err := newAppUpdater().fetchInstaller(ctx, version)
	if err != nil {
		return "", err
	}
	// The NSIS installer installs machine-wide and carries an elevation
	// manifest — plain CreateProcess fails with "requires elevation".
	// cmd's start goes through ShellExecute, which shows the UAC prompt.
	if err := exec.Command("cmd", "/C", "start", "", path).Start(); err != nil {
		return "", fmt.Errorf("could not launch the installer: %w", err)
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		wruntime.Quit(a.ctx)
	}()
	return tag, nil
}

// CheckUpdates compares the running versions against the latest releases.
func (a *App) CheckUpdates() (UpdateInfo, error) {
	out := UpdateInfo{}
	appTag, appURL, errA := latestRelease("Templetry/desktop")
	engTag, engURL, errB := latestRelease("Templetry/engine")
	if errA != nil && errB != nil {
		return out, fmt.Errorf("checking updates: %v", errA)
	}
	if errA == nil {
		out.AppLatest, out.AppURL = appTag, appURL
		cur := norm(version)
		// Dev/local builds never flag updates; /releases/latest already
		// excludes drafts and prereleases by contract.
		if cur != "dev" && !strings.HasSuffix(cur, "-local") && !strings.HasSuffix(cur, "-dev") {
			out.AppUpdate = isNewerVersion(cur, appTag)
		}
	}
	if errB == nil {
		out.EngineLatest, out.EngineURL = engTag, engURL
		cur := norm(engineVersion())
		// Skip Go pseudo-versions (branch builds) — only compare real tags.
		if cur != "unknown" && !strings.Contains(cur, "-0.20") {
			out.EngineUpdate = norm(engTag) != cur
		}
	}
	return out, nil
}
