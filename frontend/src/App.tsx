import { useEffect, useMemo, useState } from "react";
import {
    GetCatalog, GetTemplate, PreviewProject, PreviewFile, ChooseParentDir, GetLastParentDir,
    GetAuthStatus, StartGitHubLogin, Logout, GetOwners, CreateFullProject,
    ListRepos, OpenRepo, CloneRepo, GetSettings, SaveSettings,
} from "../wailsjs/go/main/App";
import "./App.css";

type Form = { form: string; name: string; path: string; status: string; description?: string };
type Parent = { key: string; label?: string; repo: string; ref: string; forms: Form[] };
type Variable = { key: string; label?: string; type?: string; pattern?: string; options?: string[]; default?: string };
type Feature = { key: string; label?: string; default?: boolean };
type Manifest = { name: string; description?: string; variables?: Variable[]; features?: Feature[] };

const LICENSES = ["", "mit", "apache-2.0", "gpl-3.0", "bsd-3-clause", "mpl-2.0", "unlicense"];

function App() {
    const [parents, setParents] = useState<Parent[]>([]);
    const [selected, setSelected] = useState("");
    const [manifest, setManifest] = useState<Manifest | null>(null);
    const [vars, setVars] = useState<Record<string, string>>({});
    const [feats, setFeats] = useState<Record<string, boolean>>({});
    const [auth, setAuth] = useState<any>({ state: "logged_out" });
    const [owners, setOwners] = useState<string[]>([]);
    const [owner, setOwner] = useState("");
    const [repoName, setRepoName] = useState("");
    const [repoPrivate, setRepoPrivate] = useState(true);
    const [license, setLicense] = useState("");
    const [parentDir, setParentDir] = useState("");
    const [previewEntries, setPreviewEntries] = useState<any[]>([]);
    const [previewSel, setPreviewSel] = useState("");
    const [previewContent, setPreviewContent] = useState("");
    const [result, setResult] = useState<{ url: string; dir: string } | null>(null);
    const [error, setError] = useState("");
    const [busy, setBusy] = useState(false);
    const [view, setView] = useState<"create" | "repos" | "settings">("create");
    const [settings, setSettings] = useState<any>({});
    const [settingsMsg, setSettingsMsg] = useState("");
    const [repos, setRepos] = useState<any[]>([]);
    const [repoFilter, setRepoFilter] = useState("");
    const [ownerFilter, setOwnerFilter] = useState("");
    const [repoMsg, setRepoMsg] = useState("");

    const loadRepos = () => {
        setBusy(true);
        ListRepos().then((r: any[]) => setRepos(r ?? []))
            .catch((e: any) => setError(String(e)))
            .finally(() => setBusy(false));
    };

    const applyUi = (s: any) => {
        const root = document.documentElement;
        root.dataset.theme = s?.uiTheme === "light" ? "light" : "dark";
        root.dataset.density = s?.uiDensity === "compact" ? "compact" : "comfortable";
        document.body.classList.toggle("layout-stacked", s?.uiLayout === "stacked");
        root.style.setProperty("--brass", s?.uiAccent || "#d9a441");
        (document.body.style as any).zoom = s?.uiScale || "1";
    };

    useEffect(() => {
        GetCatalog().then((r: any) => setParents(r.parents ?? [])).catch((e: any) => setError(String(e)));
        GetAuthStatus().then(setAuth).catch(() => {});
        GetSettings().then((s: any) => {
            applyUi(s);
            setSettings(s ?? {});
            const dir = s?.defaultParentDir || s?.lastParentDir;
            if (dir) setParentDir(dir);
            if (s?.defaultLicense) setLicense(s.defaultLicense);
            if (typeof s?.defaultPrivate === "boolean") setRepoPrivate(s.defaultPrivate);
        }).catch(() => {});
        GetLastParentDir().catch(() => {});
    }, []);

    useEffect(() => {
        if (auth.state !== "pending") return;
        const t = setInterval(() => GetAuthStatus().then(setAuth).catch(() => {}), 3000);
        return () => clearInterval(t);
    }, [auth.state]);

    useEffect(() => {
        if (auth.state === "logged_in") {
            GetOwners().then((o: string[]) => {
                setOwners(o);
                if (!owner) setOwner(settings.defaultOwner && o.includes(settings.defaultOwner) ? settings.defaultOwner : o[0] ?? "");
            }).catch(() => {});
        } else {
            setOwners([]);
            setOwner("");
        }
    }, [auth.state]);

    const pick = async (ref: string) => {
        setSelected(ref); setManifest(null); setPreviewEntries([]); setPreviewSel(""); setPreviewContent("");
        setResult(null); setError(""); setBusy(true);
        try {
            const m: Manifest = (await GetTemplate(ref)) as any;
            setManifest(m);
            const v: Record<string, string> = {};
            (m.variables ?? []).forEach((x) => { v[x.key] = x.default ?? ""; });
            setVars(v);
            const f: Record<string, boolean> = {};
            (m.features ?? []).forEach((x) => { f[x.key] = x.default ?? false; });
            setFeats(f);
        } catch (e) { setError(String(e)); } finally { setBusy(false); }
    };

    const inputs = useMemo(() => {
        const v: Record<string, string> = {};
        Object.entries(vars).forEach(([k, val]) => { if (val !== "") v[k] = val; });
        return v;
    }, [vars]);

    const targetPath = parentDir && repoName ? `${parentDir.replace(/[\\/]+$/, "")}\\${repoName}` : "";

    const preview = async () => {
        setBusy(true); setError(""); setResult(null); setPreviewSel(""); setPreviewContent("");
        try {
            setPreviewEntries((await PreviewProject(selected, inputs, feats)) as any[]);
            setTimeout(() => document.querySelector(".previewsec")?.scrollIntoView({ behavior: "smooth", block: "start" }), 50);
        }
        catch (e) { setError(String(e)); } finally { setBusy(false); }
    };

    const openPreviewFile = async (path: string) => {
        setPreviewSel(path);
        try { setPreviewContent(String(await PreviewFile(path))); }
        catch (e) { setPreviewContent(String(e)); }
    };

    const create = async () => {
        setBusy(true); setError(""); setResult(null);
        try {
            const desc = `Created with Templetry (${selected})`;
            const r: any = await CreateFullProject(selected, owner, repoName, desc, license, repoPrivate, parentDir, inputs, feats);
            setResult(r);
        } catch (e) { setError(String(e)); } finally { setBusy(false); }
    };

    return (
        <div id="shell">
            <header className="topbar">
                <div className="brand">
                    <h1>Templetry</h1>
                    <span className="tag">Project scaffolding for every platform</span>
                </div>
                <div className="session">
                    <button className={view === "settings" ? "active" : ""} onClick={() => setView("settings")}
                        title="Profile & settings">⚙ Settings</button>
                    {auth.state === "logged_in" && (
                        <>
                            <span className="who">@{auth.login}</span>
                            <button onClick={() => Logout().then(() => setAuth({ state: "logged_out" }))}>Sign out</button>
                        </>
                    )}
                    {auth.state === "pending" && (
                        <span className="pendingchip">Code: <strong>{auth.userCode}</strong></span>
                    )}
                    {(auth.state === "logged_out" || auth.state === "error") && (
                        <button className="primary"
                            onClick={() => StartGitHubLogin().then(setAuth).catch((e: any) => setError(String(e)))}>
                            Sign in with GitHub
                        </button>
                    )}
                </div>
            </header>
            <div id="app">
            <aside>
                <div className="nav">
                    <button className={view === "create" ? "active" : ""} onClick={() => setView("create")}>
                        New project
                    </button>
                    <button className={view === "repos" ? "active" : ""} disabled={auth.state !== "logged_in"}
                        onClick={() => { setView("repos"); if (!repos.length) loadRepos(); }}>
                        My repos
                    </button>
                </div>
                {view === "create" && parents.map((p) => (
                    <div key={p.key} className="parent">
                        <h2>{p.label ?? p.key}</h2>
                        {p.forms.map((f) => {
                            const ref = `${p.key}/${f.form}`;
                            const ready = !f.status || f.status === "ready";
                            return (
                                <button key={ref} className={`form ${selected === ref ? "active" : ""}`}
                                    disabled={!ready} onClick={() => pick(ref)} title={f.description}>
                                    <span>{f.form}</span>
                                    {!ready && <em>{f.status}</em>}
                                </button>
                            );
                        })}
                    </div>
                ))}
            </aside>

            <main>
                {view === "settings" && (
                    <>
                        <header><h2>Profile &amp; settings</h2></header>
                        <section>
                            <h3>Profile</h3>
                            {auth.state === "logged_in"
                                ? <p>Signed in as <strong>@{auth.login}</strong></p>
                                : <p className="hint">Not signed in.</p>}
                        </section>
                        <section>
                            <h3>Defaults</h3>
                            <div className="field">
                                <span>Repositories folder</span>
                                <input value={settings.defaultParentDir ?? ""} readOnly
                                    placeholder="Where clones and new projects go…" />
                                <button onClick={async () => {
                                    const d = await ChooseParentDir();
                                    if (d) setSettings({ ...settings, defaultParentDir: d });
                                }}>Browse…</button>
                            </div>
                            <label className="field">
                                <span>Default owner</span>
                                <select value={settings.defaultOwner ?? ""}
                                    onChange={(e) => setSettings({ ...settings, defaultOwner: e.target.value })}>
                                    <option value="">(none)</option>
                                    {owners.map((o) => <option key={o} value={o}>{o}</option>)}
                                </select>
                            </label>
                            <label className="field">
                                <span>Default license</span>
                                <select value={settings.defaultLicense ?? ""}
                                    onChange={(e) => setSettings({ ...settings, defaultLicense: e.target.value })}>
                                    {LICENSES.map((l) => <option key={l} value={l}>{l === "" ? "(none)" : l}</option>)}
                                </select>
                            </label>
                            <label className="feature">
                                <input type="checkbox" checked={settings.defaultPrivate ?? true}
                                    onChange={(e) => setSettings({ ...settings, defaultPrivate: e.target.checked })} />
                                <span>New repositories private by default</span>
                            </label>
                        </section>
                        <section>
                            <h3>Appearance</h3>
                            <label className="field">
                                <span>Theme</span>
                                <select value={settings.uiTheme ?? "dark"}
                                    onChange={(e) => { const s = { ...settings, uiTheme: e.target.value }; setSettings(s); applyUi(s); }}>
                                    <option value="dark">Dark (Ink &amp; Brass)</option>
                                    <option value="light">Light (Linen)</option>
                                </select>
                            </label>
                            <label className="field">
                                <span>Accent color</span>
                                <input type="color" value={settings.uiAccent || "#d9a441"}
                                    onChange={(e) => { const s = { ...settings, uiAccent: e.target.value }; setSettings(s); applyUi(s); }} />
                            </label>
                            <label className="field">
                                <span>Density</span>
                                <select value={settings.uiDensity ?? "comfortable"}
                                    onChange={(e) => { const s = { ...settings, uiDensity: e.target.value }; setSettings(s); applyUi(s); }}>
                                    <option value="comfortable">Comfortable</option>
                                    <option value="compact">Compact</option>
                                </select>
                            </label>
                            <label className="field">
                                <span>Interface scale</span>
                                <select value={settings.uiScale ?? "1"}
                                    onChange={(e) => { const s = { ...settings, uiScale: e.target.value }; setSettings(s); applyUi(s); }}>
                                    <option value="0.9">90%</option>
                                    <option value="1">100%</option>
                                    <option value="1.1">110%</option>
                                    <option value="1.25">125%</option>
                                </select>
                            </label>
                            <label className="field">
                                <span>Preview panel</span>
                                <select value={settings.uiLayout ?? "auto"}
                                    onChange={(e) => { const s = { ...settings, uiLayout: e.target.value }; setSettings(s); applyUi(s); }}>
                                    <option value="auto">Beside the form on wide windows</option>
                                    <option value="stacked">Always below the form</option>
                                </select>
                            </label>
                            <p className="hint">Changes apply live; Save makes them permanent.</p>
                        </section>
                        <section>
                            <h3>Catalog</h3>
                            <label className="field">
                                <span>Registry URL</span>
                                <input value={settings.registryUrl ?? ""}
                                    placeholder="(official Templetry catalog)"
                                    onChange={(e) => setSettings({ ...settings, registryUrl: e.target.value })} />
                            </label>
                            <p className="hint">Cross-device sync of these settings via your GitHub account is planned.</p>
                        </section>
                        <div className="actions">
                            <button className="primary" onClick={() => {
                                SaveSettings(settings).then(() => {
                                    setSettingsMsg("Saved.");
                                    if (settings.defaultParentDir) setParentDir(settings.defaultParentDir);
                                }).catch((e: any) => setError(String(e)));
                            }}>Save settings</button>
                        </div>
                        {settingsMsg && <pre className="output">{settingsMsg}</pre>}
                        {error && <pre className="error">{error}</pre>}
                    </>
                )}
                {view === "repos" && (
                    <>
                        <header><h2>My repositories</h2></header>
                        <div className="outrow" style={{ marginTop: 16 }}>
                            <input placeholder="Search…" value={repoFilter}
                                onChange={(e) => setRepoFilter(e.target.value)} />
                            <select value={ownerFilter} onChange={(e) => setOwnerFilter(e.target.value)}>
                                <option value="">All owners</option>
                                {[...new Set(repos.map((r) => r.owner))].map((o) => (
                                    <option key={o} value={o}>{o}</option>
                                ))}
                            </select>
                            <button onClick={loadRepos} disabled={busy}>Refresh</button>
                        </div>
                        {repoMsg && <pre className="output">{repoMsg}</pre>}
                        {error && <pre className="error">{error}</pre>}
                        <div className="repolist">
                            {repos
                                .filter((r) => !ownerFilter || r.owner === ownerFilter)
                                .filter((r) => !repoFilter || r.fullName.toLowerCase().includes(repoFilter.toLowerCase()))
                                .map((r) => (
                                    <div key={r.fullName} className={`repo ${r.archived ? "archived" : ""}`}>
                                        <div className="repoinfo">
                                            <strong>{r.fullName}</strong>
                                            <span className="meta">
                                                {r.private ? "private" : "public"}
                                                {r.language ? ` · ${r.language}` : ""}
                                                {r.archived ? " · archived" : ""}
                                                {` · ${String(r.updatedAt).slice(0, 10)}`}
                                            </span>
                                            {r.description && <span className="desc">{r.description}</span>}
                                        </div>
                                        <div className="repoactions">
                                            <button onClick={() => OpenRepo(r.htmlUrl)}>Open</button>
                                            <button disabled={busy} onClick={() => {
                                                setBusy(true); setError(""); setRepoMsg("");
                                                CloneRepo(r.cloneUrl, r.name)
                                                    .then((d: string) => setRepoMsg(`Cloned: ${d}`))
                                                    .catch((e: any) => setError(String(e)))
                                                    .finally(() => setBusy(false));
                                            }}>Clone</button>
                                        </div>
                                    </div>
                                ))}
                            {!repos.length && !busy && <div className="empty">No repositories loaded.</div>}
                        </div>
                    </>
                )}
                {view === "create" && !selected && <div className="empty">Pick a template form to start.</div>}
                {view === "create" && selected && manifest && (
                    <>
                        <header>
                            <h2>{manifest.name}</h2>
                            {manifest.description && <p>{manifest.description}</p>}
                        </header>
                        <div className="workspace">
                        <div className="formcol">

                        {(manifest.variables ?? []).length > 0 && (
                            <section>
                                <h3>Template</h3>
                                {(manifest.variables ?? []).map((v) => (
                                    <label key={v.key} className="field">
                                        <span>{v.label ?? v.key}</span>
                                        {v.type === "select" ? (
                                            <select value={vars[v.key] ?? ""}
                                                onChange={(e) => setVars({ ...vars, [v.key]: e.target.value })}>
                                                {(v.options ?? []).map((o) => <option key={o} value={o}>{o}</option>)}
                                            </select>
                                        ) : (
                                            <input value={vars[v.key] ?? ""} placeholder={v.pattern ?? ""}
                                                onChange={(e) => setVars({ ...vars, [v.key]: e.target.value })} />
                                        )}
                                    </label>
                                ))}
                                {(manifest.features ?? []).length > 0 && (
                                    <div className="features">
                                        {(manifest.features ?? []).map((f) => (
                                            <label key={f.key} className="feature">
                                                <input type="checkbox" checked={feats[f.key] ?? false}
                                                    onChange={(e) => setFeats({ ...feats, [f.key]: e.target.checked })} />
                                                <span>{f.label ?? f.key}</span>
                                            </label>
                                        ))}
                                    </div>
                                )}
                            </section>
                        )}

                        <section>
                            <h3>Repository</h3>
                            <label className="field">
                                <span>Create in</span>
                                <select value={owner} onChange={(e) => setOwner(e.target.value)}>
                                    <option value="">(local only — no GitHub repo)</option>
                                    {owners.map((o) => <option key={o} value={o}>{o}</option>)}
                                </select>
                            </label>
                            {auth.state !== "logged_in" && (
                                <p className="hint">Sign in with GitHub (sidebar) to create the repo in the cloud.</p>
                            )}
                            <label className="field">
                                <span>Name</span>
                                <input value={repoName} placeholder="my-new-repo"
                                    onChange={(e) => setRepoName(e.target.value.toLowerCase().replace(/[^a-z0-9._-]+/g, "-"))} />
                            </label>
                            {owner !== "" && (
                                <>
                                    <label className="field">
                                        <span>License</span>
                                        <select value={license} onChange={(e) => setLicense(e.target.value)}>
                                            {LICENSES.map((l) => <option key={l} value={l}>{l === "" ? "(none)" : l}</option>)}
                                        </select>
                                    </label>
                                    <label className="feature">
                                        <input type="checkbox" checked={repoPrivate}
                                            onChange={(e) => setRepoPrivate(e.target.checked)} />
                                        <span>Private repository</span>
                                    </label>
                                </>
                            )}
                        </section>

                        <section>
                            <h3>Local folder</h3>
                            <div className="outrow">
                                <input value={parentDir} placeholder="Parent folder for the project…" readOnly />
                                <button onClick={async () => setParentDir((await ChooseParentDir()) || parentDir)}>Browse…</button>
                            </div>
                            {targetPath && <p className="hint">Will create: {targetPath}</p>}
                        </section>

                        <div className="actions">
                            <button disabled={busy} onClick={preview}>Preview</button>
                            <button disabled={busy || !repoName || !parentDir} className="primary" onClick={create}>
                                {owner ? "Create repo & project" : "Create project"}
                            </button>
                        </div>

                        {busy && <p className="hint">Working…</p>}
                        {error && <pre className="error">{error}</pre>}
                        {result && (
                            <pre className="output">
                                {result.url ? `Repository: ${result.url}\n` : ""}Local: {result.dir}
                            </pre>
                        )}
                        </div>
                        <div className="sidecol">
                            {previewEntries.length > 0 ? (
                                <section className="previewsec">
                                    <h3>Preview — {previewEntries.length} files</h3>
                                    <div className="preview">
                                        <div className="ptree">
                                            {previewEntries.map((e) => (
                                                <button key={e.path}
                                                    className={`pfile ${previewSel === e.path ? "active" : ""}`}
                                                    onClick={() => openPreviewFile(e.path)}>
                                                    <span>{e.path}</span>
                                                    <em>{e.binary ? "bin" : `${e.size}b`}</em>
                                                </button>
                                            ))}
                                        </div>
                                        <pre className="pcontent">
                                            {previewSel ? previewContent : "Select a file to inspect its rendered content."}
                                        </pre>
                                    </div>
                                </section>
                            ) : (
                                <div className="previewempty">Preview shows the rendered project here.</div>
                            )}
                        </div>
                        </div>
                    </>
                )}
                {view === "create" && selected && !manifest && !error && <div className="empty">Fetching template…</div>}
            </main>
            </div>
        </div>
    );
}

export default App;
