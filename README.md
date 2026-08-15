# Templetry Desktop

Manage the [Templetry](https://github.com/Templetry) engine from a native app: sign in to your forges, browse the catalog, create ready-to-work repos and keep them up to date.

Built with [Wails](https://wails.io): a Go backend that imports the engine packages directly (no IPC, no sidecar — [ADR-0012](https://github.com/Templetry/wiki/blob/main/adr/0012-desktop-app-wails.md)) and a React/TypeScript frontend.

## The three sections

- **Build** — repo-first creation: browse the catalog by taxonomy (kind chips and a search across every axis), pick a form, fill the manifest-driven dynamic form, preview the render, **verify** it (the rendered project is built inside the manifest's container, so no toolchain is installed here), create the repo on any signed-in forge (or push to a pasted remote, or stay local-only) and get a ready-to-work clone.
- **Cloud** — your repositories across every signed-in account and org, on GitHub, GitLab and Gitea/Forgejo alike: clones already on disk are cross-linked, and every repo opens a state preview — description, languages, branches, latest CI runs or pipelines, README and docs rendered as markdown. A repo the engine can render is flagged, and its manifests are read, so each detected form reports its name, description and taxonomy.
- **Local** — every repository under your repositories folder, found recursively and organized by folder: Templetry projects with their provenance and update cycle, plain git repos with branch and remote, a **pieces panel**, and a per-repo preview — branches, remotes, last commit, working-tree state and docs.

## Also in the box

- **Accounts on several forges**: GitHub via OAuth device flow, GitLab and Gitea/Forgejo via a personal access token stored in the OS keyring — plus **BYOR**, which pushes to *any* git host from a pasted repository URL ([ADR-0015](https://github.com/Templetry/wiki/blob/main/adr/0015-multi-forge-foundation.md)).
- **Templates from any forge**: the catalog says where each parent lives, so forms hosted on GitLab or Gitea render exactly like GitHub ones — and private templates read with the token of the account on that host.
- **Template updates**: drift detection → assisted update → three-way merge, anchored on the answers file. Drift covers applied **pieces** too, each against its own source.
- **Pieces**, form-local and **common** ones from the shared catalogs ([ADR-0016](https://github.com/Templetry/wiki/blob/main/adr/0016-common-pieces.md)), with their own variables.
- **Multi-catalog** support — any registry.json (schema v2) plugs into the sidebar.
- **In-app updater** checking published stable releases (Windows).

Builds for **Windows** (installer + portable), **Linux** and **macOS** (universal).

## Install

Download the installer or the portable exe from [Releases](https://github.com/Templetry/desktop/releases). Windows needs the WebView2 runtime (preinstalled on Windows 11).

Full walkthrough: the wiki's [desktop guide](https://github.com/Templetry/wiki/blob/main/guide/desktop.md).

## Development

```sh
wails dev     # live development (Vite hot reload; Go bindings on http://localhost:34115)
wails build   # redistributable production build
go test ./...  # unit tests
```

Configuration lives in `wails.json` ([reference](https://wails.io/docs/reference/project-config)). For GitHub auth in development, set `TEMPLETRY_GH_CLIENT_ID` to your own OAuth App client id (device flow enabled).

### Live forge checks

The Cloud preview for GitLab and Gitea is a set of assumptions about JSON field names and endpoint paths — the kind of thing that compiles, passes every unit test, and is wrong.

```sh
go test -tags liveapi -run Live -v ./...
```

These read **public** repositories with no token, so they need no account. They are behind a build tag because they reach servers nobody here controls: a red result may mean the API changed, or merely that codeberg.org is down.

They earned their keep on the first run — GitLab paginates a recursive tree at 100 entries and walks depth-first, so a large repository's first page is all directories and the docs list came back empty. Both forges also truncate large trees silently; the cap is now a named constant instead of an accident.
