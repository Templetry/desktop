package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
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
		out.AppUpdate = cur != "dev" && norm(appTag) != cur
	}
	if errB == nil {
		out.EngineLatest, out.EngineURL = engTag, engURL
		cur := norm(engineVersion())
		out.EngineUpdate = cur != "unknown" && norm(engTag) != cur
	}
	return out, nil
}
