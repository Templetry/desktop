package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		name, current, latest string
		want                  bool
	}{
		{"newer minor", "v1.14.0", "v1.15.0", true},
		{"newer patch", "1.14.0", "v1.14.1", true},
		{"newer major", "v1.99.9", "v2.0.0", true},
		{"numeric not lexical", "v1.9.0", "v1.10.0", true},
		{"equal", "v1.14.0", "v1.14.0", false},
		{"equal without prefix", "1.14.0", "v1.14.0", false},
		{"older minor (downgrade)", "v1.14.0", "v1.13.0", false},
		{"older major (downgrade)", "v2.0.0", "v1.99.99", false},
		{"final beats its prerelease", "v1.15.0-rc1", "v1.15.0", true},
		{"prerelease does not beat final", "v1.15.0", "v1.15.0-rc1", false},
		{"dev build never updates", "dev", "v9.9.9", false},
		{"local build never updates", "v1.0.0-local", "v1.0.0", false},
		{"garbage latest", "v1.14.0", "latest", false},
		{"empty latest", "v1.14.0", "", false},
		{"two components", "v1.14.0", "v1.15", false},
		{"four components", "v1.14.0", "v1.15.0.1", false},
		{"negative", "v1.14.0", "v1.-1.0", false},
		{"leading zero", "v1.14.0", "v1.015.0", false},
		{"garbage current", "banana", "v1.15.0", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isNewerVersion(c.current, c.latest); got != c.want {
				t.Fatalf("isNewerVersion(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
			}
		})
	}
}

func TestFindSum(t *testing.T) {
	good := strings.Repeat("ab", 32)
	cases := []struct {
		name, sums, file string
		want             string
		wantErr          bool
	}{
		{"match", good + "  a.exe\n", "a.exe", good, false},
		{"binary marker", good + " *a.exe\n", "a.exe", good, false},
		{"uppercase normalised", strings.ToUpper(good) + "  a.exe\n", "a.exe", good, false},
		{"missing entry", good + "  other.exe\n", "a.exe", "", true},
		{"short hash", "abcd  a.exe\n", "a.exe", "", true},
		{"non-hex hash", strings.Repeat("zz", 32) + "  a.exe\n", "a.exe", "", true},
		{"empty", "", "a.exe", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := findSum(c.sums, c.file)
			if (err != nil) != c.wantErr || got != c.want {
				t.Fatalf("findSum = %q, %v; want %q, err=%v", got, err, c.want, c.wantErr)
			}
		})
	}
}

// fakeRelease serves a release whose assets live on the same test server.
type fakeRelease struct {
	tag        string
	installer  []byte
	sums       func(installerSum string) string // nil: no SHA256SUMS asset
	releaseSC  int                              // status of the metadata call (0 = 200)
	downloadSC int                              // status of the installer download (0 = 200)
	noInstall  bool
}

func (f fakeRelease) serve(t *testing.T) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(f.installer)
	name := "Templetry-" + f.tag + "-windows-installer.exe"
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			if f.releaseSC != 0 {
				http.Error(w, "nope", f.releaseSC)
				return
			}
			assets := ""
			if !f.noInstall {
				assets += fmt.Sprintf(`{"name":%q,"browser_download_url":%q}`, name, srv.URL+"/dl/"+name)
			}
			if f.sums != nil {
				if assets != "" {
					assets += ","
				}
				assets += fmt.Sprintf(`{"name":"SHA256SUMS","browser_download_url":%q}`, srv.URL+"/dl/SHA256SUMS")
			}
			fmt.Fprintf(w, `{"tag_name":%q,"assets":[%s]}`, f.tag, assets)
		case strings.HasSuffix(r.URL.Path, "/SHA256SUMS"):
			fmt.Fprint(w, f.sums(hex.EncodeToString(sum[:])))
		case strings.HasSuffix(r.URL.Path, name):
			if f.downloadSC != 0 {
				http.Error(w, "nope", f.downloadSC)
				return
			}
			w.Write(f.installer)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func goodSums(name string) func(string) string {
	return func(sum string) string { return sum + "  " + name + "\n" }
}

func TestFetchInstaller(t *testing.T) {
	payload := []byte("MZ-not-a-real-installer")
	name := "Templetry-v1.15.0-windows-installer.exe"

	cases := []struct {
		name    string
		current string
		rel     fakeRelease
		max     int64
		wantErr string // substring; "" = success
	}{
		{"hash matches", "v1.14.0", fakeRelease{tag: "v1.15.0", installer: payload, sums: goodSums(name)}, 0, ""},
		{"hash mismatch", "v1.14.0", fakeRelease{tag: "v1.15.0", installer: payload,
			sums: func(string) string { return strings.Repeat("0", 64) + "  " + name + "\n" }}, 0, "checksum mismatch"},
		{"missing sums entry", "v1.14.0", fakeRelease{tag: "v1.15.0", installer: payload,
			sums: func(s string) string { return s + "  something-else.exe\n" }}, 0, "no valid entry"},
		{"no SHA256SUMS asset", "v1.14.0", fakeRelease{tag: "v1.15.0", installer: payload}, 0, "no SHA256SUMS"},
		{"no installer asset", "v1.14.0", fakeRelease{tag: "v1.15.0", installer: payload, sums: goodSums(name), noInstall: true}, 0, "no Windows installer"},
		{"metadata non-200", "v1.14.0", fakeRelease{tag: "v1.15.0", installer: payload, sums: goodSums(name), releaseSC: 500}, 0, "HTTP 500"},
		{"metadata rate limited", "v1.14.0", fakeRelease{tag: "v1.15.0", installer: payload, sums: goodSums(name), releaseSC: 403}, 0, "HTTP 403"},
		{"download non-200", "v1.14.0", fakeRelease{tag: "v1.15.0", installer: payload, sums: goodSums(name), downloadSC: 404}, 0, "HTTP 404"},
		{"oversize download", "v1.14.0", fakeRelease{tag: "v1.15.0", installer: payload, sums: goodSums(name)}, 8, "limit"},
		{"downgrade rejected", "v1.16.0", fakeRelease{tag: "v1.15.0", installer: payload, sums: goodSums(name)}, 0, "not newer"},
		{"equal rejected", "v1.15.0", fakeRelease{tag: "v1.15.0", installer: payload, sums: goodSums(name)}, 0, "not newer"},
		{"dev build rejected", "dev", fakeRelease{tag: "v1.15.0", installer: payload, sums: goodSums(name)}, 0, "not newer"},
		{"garbage tag rejected", "v1.14.0", fakeRelease{tag: "nightly", installer: payload, sums: goodSums("Templetry-nightly-windows-installer.exe")}, 0, "not newer"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMP", tmp)
			t.Setenv("TEMP", tmp)
			t.Setenv("TMPDIR", tmp)
			srv := c.rel.serve(t)
			u := &appUpdater{Repo: "x/y", APIURL: srv.URL, Client: srv.Client(), MaxInstall: maxInstallerSize}
			if c.max != 0 {
				u.MaxInstall = c.max
			}
			path, tag, err := u.fetchInstaller(context.Background(), c.current)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, c.wantErr)
				}
				if path != "" {
					t.Fatalf("path %q returned alongside an error", path)
				}
				// Nothing unverified may be left behind.
				left, _ := os.ReadDir(tmp)
				if len(left) != 0 {
					t.Fatalf("temp dir not cleaned up: %v", left)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tag != c.rel.tag {
				t.Fatalf("tag = %q", tag)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != string(payload) {
				t.Fatalf("installer content wrong: %v", err)
			}
			if filepath.Dir(path) == tmp {
				t.Fatalf("installer should live in its own per-run directory, got %s", path)
			}
		})
	}
}

func TestFetchInstallerUniqueDirs(t *testing.T) {
	payload := []byte("x")
	name := "Templetry-v1.15.0-windows-installer.exe"
	srv := fakeRelease{tag: "v1.15.0", installer: payload, sums: goodSums(name)}.serve(t)
	u := &appUpdater{Repo: "x/y", APIURL: srv.URL, Client: srv.Client(), MaxInstall: maxInstallerSize}
	a, _, err := u.fetchInstaller(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := u.fetchInstaller(context.Background(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Dir(a))
	defer os.RemoveAll(filepath.Dir(b))
	if filepath.Dir(a) == filepath.Dir(b) {
		t.Fatalf("two runs shared a directory: %s", filepath.Dir(a))
	}
}
