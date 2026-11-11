package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// maxInstallerSize caps the installer download (the real one is a few
	// tens of MiB).
	maxInstallerSize = 512 << 20
	maxSumsSize      = 1 << 20
	maxReleaseJSON   = 4 << 20

	installerSuffix = "-windows-installer.exe"
	sumsAssetName   = "SHA256SUMS"
	desktopRepo     = "Templetry/desktop"
)

// appUpdater finds the latest desktop release and downloads its installer,
// handing back a file only after its SHA-256 matches the release's
// SHA256SUMS. It never launches anything.
type appUpdater struct {
	Repo       string
	APIURL     string // https://api.github.com by default
	Client     *http.Client
	MaxInstall int64
}

func newAppUpdater() *appUpdater {
	return &appUpdater{
		Repo:       desktopRepo,
		APIURL:     "https://api.github.com",
		Client:     &http.Client{Timeout: 15 * time.Minute},
		MaxInstall: maxInstallerSize,
	}
}

type appRelease struct {
	Tag    string
	Assets map[string]string // asset name -> download URL
}

func (u *appUpdater) latest(ctx context.Context) (*appRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.APIURL+"/repos/"+u.Repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release lookup: HTTP %d", resp.StatusCode)
	}
	var body struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxReleaseJSON)).Decode(&body); err != nil {
		return nil, err
	}
	rel := &appRelease{Tag: body.TagName, Assets: map[string]string{}}
	for _, a := range body.Assets {
		rel.Assets[a.Name] = a.URL
	}
	return rel, nil
}

// fetchInstaller verifies that the latest release is strictly newer than
// current, then downloads its installer into a fresh private directory and
// checks it against SHA256SUMS. On any failure nothing is left on disk.
// It returns the installer path and the release tag.
func (u *appUpdater) fetchInstaller(ctx context.Context, current string) (path, tag string, err error) {
	rel, err := u.latest(ctx)
	if err != nil {
		return "", "", err
	}
	if !isNewerVersion(current, rel.Tag) {
		return "", "", fmt.Errorf("release %q is not newer than the running version %q; not updating", rel.Tag, current)
	}
	name, url := "", ""
	for n, a := range rel.Assets {
		if strings.HasSuffix(n, installerSuffix) {
			name, url = n, a
			break
		}
	}
	if name == "" {
		return "", "", errors.New("the latest release has no Windows installer asset")
	}
	sumsURL, ok := rel.Assets[sumsAssetName]
	if !ok {
		return "", "", errors.New("the latest release has no SHA256SUMS; refusing to install an unverifiable file")
	}
	sums, err := u.fetchSmall(ctx, sumsURL, maxSumsSize)
	if err != nil {
		return "", "", fmt.Errorf("downloading SHA256SUMS: %w", err)
	}
	want, err := findSum(string(sums), name)
	if err != nil {
		return "", "", err
	}

	dir, err := os.MkdirTemp("", "Templetry-update-")
	if err != nil {
		return "", "", err
	}
	path = filepath.Join(dir, filepath.Base(name))
	got, err := u.downloadTo(ctx, url, path)
	if err != nil {
		os.RemoveAll(dir)
		return "", "", err
	}
	if got != want {
		os.RemoveAll(dir)
		return "", "", fmt.Errorf("checksum mismatch for %s: expected %s, got %s", name, want, got)
	}
	return path, rel.Tag, nil
}

func (u *appUpdater) fetchSmall(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errors.New("response too large")
	}
	return b, nil
}

// downloadTo streams url into path, enforcing the size cap, and returns the
// lowercase hex SHA-256 of what was written.
func (u *appUpdater) downloadTo(ctx context.Context, url, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := u.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("installer download: HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > u.MaxInstall {
		return "", fmt.Errorf("installer is %d bytes, over the %d byte limit", resp.ContentLength, u.MaxInstall)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, u.MaxInstall+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if n > u.MaxInstall {
		return "", fmt.Errorf("installer exceeds the %d byte limit", u.MaxInstall)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// findSum reads "<hex>  <name>" lines (sha256sum format).
func findSum(sums, name string) (string, error) {
	sc := bufio.NewScanner(strings.NewReader(sums))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name && len(f[0]) == 64 {
			if _, err := hex.DecodeString(f[0]); err == nil {
				return strings.ToLower(f[0]), nil
			}
		}
	}
	return "", fmt.Errorf("SHA256SUMS has no valid entry for %s", name)
}

type semver struct {
	n   [3]int
	pre string
}

func parseSemver(v string) (semver, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	var out semver
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		if v[i] == '-' {
			out.pre = v[i+1:]
			if j := strings.Index(out.pre, "+"); j >= 0 {
				out.pre = out.pre[:j]
			}
		}
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p != strconv.Itoa(n) {
			return out, false
		}
		out.n[i] = n
	}
	return out, true
}

// isNewerVersion reports whether latest is strictly higher than current.
// Equal, older, and unparsable versions (including "dev") are all rejected;
// a pre-release sorts below its release.
func isNewerVersion(current, latest string) bool {
	c, okC := parseSemver(current)
	l, okL := parseSemver(latest)
	if !okC || !okL || c.pre == "local" || c.pre == "dev" { // dev builds never update
		return false
	}
	for i := 0; i < 3; i++ {
		if l.n[i] != c.n[i] {
			return l.n[i] > c.n[i]
		}
	}
	// Same numbers: only a final release beats a pre-release of itself.
	return c.pre != "" && l.pre == ""
}
