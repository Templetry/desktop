# Templetry Desktop

Manage the [Templetry](https://github.com/Templetry) engine from a native app: sign in with GitHub, browse the catalog, create ready-to-work repos and keep them up to date.

Built with [Wails](https://wails.io): a Go backend that imports the engine packages directly (no IPC, no sidecar — [ADR-0012](https://github.com/Templetry/wiki/blob/main/adr/0012-desktop-app-wails.md)) and a React/TypeScript frontend.

## Features

- **GitHub sign-in** via OAuth device flow (scopes `repo workflow`) — no server, no hosted OAuth.
- **Repo-first creation**: pick a form from the catalog, fill the manifest-driven dynamic form, preview the render, create the repo on GitHub.
- **Repo & project management**, multi-catalog support.
- **Template updates**: drift detection → assisted update → three-way merge, anchored on the answers file.
- **In-app updater** checking published stable releases.

## Install

Download the installer or the portable exe from [Releases](https://github.com/Templetry/desktop/releases). Windows needs the WebView2 runtime (preinstalled on Windows 11).

## Development

```sh
wails dev     # live development (Vite hot reload; Go bindings on http://localhost:34115)
wails build   # redistributable production build
```

Configuration lives in `wails.json` ([reference](https://wails.io/docs/reference/project-config)). For GitHub auth in development, set `TEMPLETRY_GH_CLIENT_ID` to your own OAuth App client id (device flow enabled).
