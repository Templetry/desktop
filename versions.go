package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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
func (a *App) GetVersions() VersionInfo {
	return VersionInfo{App: version, Engine: engineVersion()}
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
// launches it and quits the app so the installer can replace it.
func (a *App) InstallAppUpdate() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/Templetry/desktop/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var rel struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	url := ""
	for _, as := range rel.Assets {
		if strings.HasSuffix(as.Name, "-windows-installer.exe") {
			url = as.URL
			break
		}
	}
	if url == "" {
		return "", fmt.Errorf("the latest release has no Windows installer asset")
	}
	dl, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer dl.Body.Close()
	tmp, err := os.CreateTemp("", "Templetry-update-*.exe")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(tmp, dl.Body); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()
	if err := exec.Command(tmp.Name()).Start(); err != nil {
		return "", fmt.Errorf("could not launch the installer: %w", err)
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		wruntime.Quit(a.ctx)
	}()
	return rel.TagName, nil
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
			out.AppUpdate = norm(appTag) != cur
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
