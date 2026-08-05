import { useEffect, useMemo, useState } from "react";
import {
    GetCatalogs, GetTemplate, PreviewProject, PreviewFile, ChooseParentDir, GetLastParentDir,
    GetAuthStatus, StartGitHubLogin, Logout, GetOwners, CreateFullProject,
    ListRepos, OpenRepo, CloneRepo, GetSettings, SaveSettings, ExportSettings, ImportSettings,
    ScanProjects, OpenFolder, GetVersions, CheckUpdates, CheckDrift,
    PreviewUpdate, UpdateFileContent, ApplyUpdate,
} from "../wailsjs/go/main/App";
import "./App.css";

type Form = { form: string; name: string; path: string; status: string; description?: string };
type Parent = { key: string; label?: string; repo: string; ref: string; forms: Form[] };
type Variable = { key: string; label?: string; type?: string; pattern?: string; options?: string[]; default?: string };
type Feature = { key: string; label?: string; default?: boolean };
type Manifest = { name: string; description?: string; variables?: Variable[]; features?: Feature[] };

const LICENSES = ["", "mit", "apache-2.0", "gpl-3.0", "bsd-3-clause", "mpl-2.0", "unlicense"];

function App() {
    const [catalogs, setCatalogs] = useState<any[]>([]);
    const [selectedCat, setSelectedCat] = useState("");
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
    const [view, setView] = useState<"create" | "repos" | "settings" | "projects">("create");
    const [collapsed, setCollapsed] = useState(localStorage.getItem("tpl.sidebar") === "1");
    const toggleSidebar = () => {
        const v = !collapsed;
        setCollapsed(v);
        localStorage.setItem("tpl.sidebar", v ? "1" : "0");
    };
    const [settings, setSettings] = useState<any>({});
    const [settingsMsg, setSettingsMsg] = useState("");
    const [repos, setRepos] = useState<any[]>([]);
    const [repoFilter, setRepoFilter] = useState("");
    const [ownerFilter, setOwnerFilter] = useState("");
    const [repoMsg, setRepoMsg] = useState("");

    const [projects, setProjects] = useState<any[]>([]);
    const [projFilter, setProjFilter] = useState("");
    const [versions, setVersions] = useState<any>({});
    const [updates, setUpdates] = useState<any>(null);
    const [updMsg, setUpdMsg] = useState("");

    const checkUpdates = (announce: boolean) => {
        CheckUpdates().then((u: any) => {
            setUpdates(u);
            if (announce) {
                setUpdMsg(u.appUpdate || u.engineUpdate
                    ? "Updates available - see below."
                    : "Everything is up to date.");
            }
        }).catch((e: any) => { if (announce) setError(String(e)); });
    };

    const [drifts, setDrifts] = useState<Record<string, string>>({});
    const [updPrev, setUpdPrev] = useState<any>(null);
    const [updSel, setUpdSel] = useState("");
    const [updContent, setUpdContent] = useState("");

    const previewUpdate = (dir: string) => {
        setBusy(true); setError(""); setUpdPrev(null); setUpdSel(""); setUpdContent("");
        PreviewUpdate(dir).then((p: any) => {
            setUpdPrev(p);
            setTimeout(() => document.getElementById("updpanel")?.scrollIntoView({ behavior: "smooth" }), 60);
        }).catch((e: any) => setError(String(e))).finally(() => setBusy(false));
    };

    const applyUpdate = () => {
        setBusy(true); setError("");
        ApplyUpdate().then((n: number) => {
            setRepoMsg(`Update applied: ${n} files written. Review with git before committing.`);
            setUpdPrev(null);
            loadProjects();
        }).catch((e: any) => setError(String(e))).finally(() => setBusy(false));
    };

    const loadProjects = () => {
        setBusy(true);
        ScanProjects().then((p: any[]) => setProjects(p ?? []))
            .catch((e: any) => setError(String(e)))
            .finally(() => setBusy(false));
        CheckDrift().then((list: any[]) =>
            setDrifts(Object.fromEntries((list ?? []).map((d: any) => [d.dir, d.latest]))))
            .catch(() => {});
    };

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

    const switchView = (v: "create" | "repos" | "settings" | "projects") => {
        setView(v);
        setError("");
        setSettingsMsg("");
        setRepoMsg("");
    };

    useEffect(() => {
        if (!settingsMsg) return;
        const t = setTimeout(() => setSettingsMsg(""), 5000);
        return () => clearTimeout(t);
    }, [settingsMsg]);

    useEffect(() => {
        if (!repoMsg) return;
        const t = setTimeout(() => setRepoMsg(""), 5000);
        return () => clearTimeout(t);
    }, [repoMsg]);

    useEffect(() => {
        if (!error) return;
        const t = setTimeout(() => setError(""), 8000);
        return () => clearTimeout(t);
    }, [error]);

    const loadCatalogs = () => {
        GetCatalogs().then((cs: any[]) => setCatalogs(cs ?? [])).catch((e: any) => setError(String(e)));
    };

    useEffect(() => {
        loadCatalogs();
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
        GetVersions().then(setVersions).catch(() => {});
        checkUpdates(false);
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

    const pick = async (cat: string, ref: string) => {
        setSelectedCat(cat);
        setSelected(ref); setManifest(null); setPreviewEntries([]); setPreviewSel(""); setPreviewContent("");
        setResult(null); setError(""); setBusy(true);
        try {
            const m: Manifest = (await GetTemplate(cat, ref)) as any;
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
            setPreviewEntries((await PreviewProject(selectedCat, selected, inputs, feats)) as any[]);
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
            const r: any = await CreateFullProject(selectedCat, selected, owner, repoName, desc, license, repoPrivate, parentDir, inputs, feats);
            setResult(r);
        } catch (e) { setError(String(e)); } finally { setBusy(false); }
    };

    return (
        <div id="shell">
            <header className="topbar">
                <div className="brand">
                    <h1>Templetry</h1>
                    <span className="ver">v{versions.app ?? "dev"}</span>
                    <span className="tag">Project scaffolding for every platform</span>
                </div>
                <div className="session">
                    {updates && (updates.appUpdate || updates.engineUpdate) && (
                        <button className="updbtn" title="A new version is available"
                            onClick={() => { switchView("settings"); setTimeout(() => document.getElementById("sec-about")?.scrollIntoView({ behavior: "smooth" }), 60); }}>
                            ● Update available
                        </button>
                    )}
                    <button className={view === "settings" ? "active" : ""} onClick={() => switchView("settings")}
                        title="Profile & settings">⚙ Settings</button>
                    {auth.state === "logged_in" && (
                        <>
                            {auth.avatar && <img className="avatar" src={auth.avatar} alt="" />}
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
            <aside className={collapsed ? "collapsed" : ""}>
                <button className="collapse" onClick={toggleSidebar}
                    title={collapsed ? "Expand sidebar" : "Collapse sidebar"}>
                    {collapsed ? "»" : "«"}
                </button>
                {collapsed ? (
                    <div className="rail">
                        <button className={view === "create" ? "active" : ""} title="New project"
                            onClick={() => switchView("create")}>+</button>
                        <button className={view === "repos" ? "active" : ""} title="My repos"
                            disabled={auth.state !== "logged_in"}
                            onClick={() => { switchView("repos"); if (!repos.length) loadRepos(); }}>▤</button>
                        <button className={view === "projects" ? "active" : ""} title="My projects"
                            onClick={() => { switchView("projects"); loadProjects(); }}>▣</button>
                    </div>
                ) : (
                    <>
                        <div className="nav">
                            <button className={view === "create" ? "active" : ""} onClick={() => switchView("create")}>
                                New project
                            </button>
                            <button className={view === "repos" ? "active" : ""} disabled={auth.state !== "logged_in"}
                                onClick={() => { switchView("repos"); if (!repos.length) loadRepos(); }}>
                                My repos
                            </button>
                            <button className={view === "projects" ? "active" : ""}
                                onClick={() => { switchView("projects"); loadProjects(); }}>
                                My projects
                            </button>
                        </div>
                        {view === "projects" && (
                            <div className="parent">
                                <h2>Templates<em>{projects.length}</em></h2>
                                <button className={`form ${projFilter === "" ? "active" : ""}`}
                                    onClick={() => setProjFilter("")}>
                                    <span>All</span>
                                    <em>{projects.length}</em>
                                </button>
                                {[...new Set(projects.map((p) => p.template))].map((t) => (
                                    <button key={t} className={`form ${projFilter === t ? "active" : ""}`}
                                        onClick={() => setProjFilter(t)}>
                                        <span>{t}</span>
                                        <em>{projects.filter((p) => p.template === t).length}</em>
                                    </button>
                                ))}
                            </div>
                        )}
                        {view === "settings" && (
                            <div className="parent">
                                <h2>Sections</h2>
                                {["Profile", "Defaults", "Appearance", "Catalogs", "About"].map((s) => (
                                    <button key={s} className="form"
                                        onClick={() => document.getElementById("sec-" + s.toLowerCase())
                                            ?.scrollIntoView({ behavior: "smooth", block: "start" })}>
                                        <span>{s}</span>
                                    </button>
                                ))}
                            </div>
                        )}
                        {view === "repos" && (
                            <div className="parent">
                                <h2>Owners<em>{repos.length}</em></h2>
                                <button className={`form ${ownerFilter === "" ? "active" : ""}`}
                                    onClick={() => setOwnerFilter("")}>
                                    <span>All</span>
                                    <em>{repos.length}</em>
                                </button>
                                {[...new Set(repos.map((r) => r.owner))].map((o) => (
                                    <button key={o} className={`form owner ${ownerFilter === o ? "active" : ""}`}
                                        onClick={() => setOwnerFilter(o)}>
                                        <span>
                                            <img className="avatar sm" alt=""
                                                src={repos.find((r) => r.owner === o)?.avatarUrl} />
                                            {o}
                                        </span>
                                        <em>{repos.filter((r) => r.owner === o).length}</em>
                                    </button>
                                ))}
                            </div>
                        )}
                        {view === "create" && catalogs.map((c) => (
                            <div key={c.name} className="catalog">
                                <div className="cathead">
                                    <span>{c.name}</span>
                                    {c.official && <em className="badge">official</em>}
                                </div>
                                {c.error && <p className="caterr">{c.error}</p>}
                                {(c.parents ?? []).map((p: Parent) => (
                                    <div key={c.name + p.key} className="parent">
                                        <h2>{p.label ?? p.key}<em>{p.forms.length}</em></h2>
                                        {p.forms.map((f) => {
                                            const ref = `${p.key}/${f.form}`;
                                            const active = selectedCat === c.name && selected === ref;
                                            const ready = !f.status || f.status === "ready";
                                            return (
                                                <button key={ref} className={`form ${active ? "active" : ""}`}
                                                    disabled={!ready} onClick={() => pick(c.name, ref)} title={f.description}>
                                                    <span>{f.form}</span>
                                                    {!ready && <em>{f.status}</em>}
                                                </button>
                                            );
                                        })}
                                    </div>
                                ))}
                            </div>
                        ))}
                    </>
                )}
            </aside>

            <main>
                {view === "settings" && (
                    <>
                        <header><h2>Profile &amp; settings</h2></header>
                        <div className="settingsgrid">
                        <section className="span2" id="sec-profile">
                            <h3>Profile</h3>
                            {auth.state === "logged_in"
                                ? <p>Signed in as <strong>@{auth.login}</strong></p>
                                : <p className="hint">Not signed in.</p>}
                        </section>
                        <section id="sec-defaults">
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
                        <section id="sec-appearance">
                            <h3>Appearance</h3>
                            <div className="field">
                                <span>Theme</span>
                                <div className="seg">
                                    {[["dark", "Dark (Ink & Brass)"], ["light", "Light (Linen)"]].map(([v, l]) => (
                                        <button key={v} className={(settings.uiTheme || "dark") === v ? "on" : ""}
                                            onClick={() => { const s = { ...settings, uiTheme: v }; setSettings(s); applyUi(s); }}>
                                            {l}
                                        </button>
                                    ))}
                                </div>
                            </div>
                            <label className="field">
                                <span>Accent color</span>
                                <input type="color" value={settings.uiAccent || "#d9a441"}
                                    onChange={(e) => { const s = { ...settings, uiAccent: e.target.value }; setSettings(s); applyUi(s); }} />
                            </label>
                            <div className="field">
                                <span>Density</span>
                                <div className="seg">
                                    {[["comfortable", "Comfortable"], ["compact", "Compact"]].map(([v, l]) => (
                                        <button key={v} className={(settings.uiDensity || "comfortable") === v ? "on" : ""}
                                            onClick={() => { const s = { ...settings, uiDensity: v }; setSettings(s); applyUi(s); }}>
                                            {l}
                                        </button>
                                    ))}
                                </div>
                            </div>
                            <label className="field">
                                <span>Interface scale</span>
                                <input type="range" min="0.8" max="1.3" step="0.05"
                                    value={parseFloat(settings.uiScale || "1")}
                                    onChange={(e) => { const s = { ...settings, uiScale: e.target.value }; setSettings(s); applyUi(s); }} />
                                <em className="scaleval">{Math.round(parseFloat(settings.uiScale || "1") * 100)}%</em>
                            </label>
                            <div className="field">
                                <span>Preview panel</span>
                                <div className="seg">
                                    {[["auto", "Beside the form"], ["stacked", "Below the form"]].map(([v, l]) => (
                                        <button key={v} className={(settings.uiLayout || "auto") === v ? "on" : ""}
                                            onClick={() => { const s = { ...settings, uiLayout: v }; setSettings(s); applyUi(s); }}>
                                            {l}
                                        </button>
                                    ))}
                                </div>
                            </div>
                            <p className="hint">Changes apply live; Save makes them permanent.</p>
                        </section>
                        <section className="span2" id="sec-catalogs">
                            <h3>Catalogs</h3>
                            <div className="catrow">
                                <span className="catname">Templetry <em className="badge">official</em></span>
                                <input value="Built-in default catalog" readOnly />
                            </div>
                            {(settings.catalogs ?? []).map((c: any, i: number) => (
                                <div key={i} className="catrow">
                                    <input className="catnameinput" placeholder="Name" value={c.name ?? ""}
                                        onChange={(e) => {
                                            const cs = [...(settings.catalogs ?? [])];
                                            cs[i] = { ...cs[i], name: e.target.value };
                                            setSettings({ ...settings, catalogs: cs });
                                        }} />
                                    <input placeholder="https://…/registry.json (or a local file path)" value={c.url ?? ""}
                                        onChange={(e) => {
                                            const cs = [...(settings.catalogs ?? [])];
                                            cs[i] = { ...cs[i], url: e.target.value };
                                            setSettings({ ...settings, catalogs: cs });
                                        }} />
                                    <button onClick={() => setSettings({
                                        ...settings,
                                        catalogs: (settings.catalogs ?? []).filter((_: any, j: number) => j !== i),
                                    })}>Remove</button>
                                </div>
                            ))}
                            <button onClick={() => setSettings({
                                ...settings,
                                catalogs: [...(settings.catalogs ?? []), { name: "", url: "" }],
                            })}>+ Add catalog</button>
                            <p className="hint">Any registry.json (schema v2) works — yours, your team's, anyone's. Save to reload the sidebar.</p>
                        </section>
                        <div className="actions">
                            <button className="primary" onClick={() => {
                                SaveSettings(settings).then(() => {
                                    setSettingsMsg("Saved.");
                                    if (settings.defaultParentDir) setParentDir(settings.defaultParentDir);
                                    loadCatalogs();
                                }).catch((e: any) => setError(String(e)));
                            }}>Save settings</button>
                            <button onClick={() => {
                                ExportSettings().then((p: string) => p && setSettingsMsg(`Exported to ${p}`))
                                    .catch((e: any) => setError(String(e)));
                            }}>Export…</button>
                            <button onClick={() => {
                                ImportSettings().then((s: any) => {
                                    setSettings(s ?? {});
                                    applyUi(s);
                                    if (s?.defaultParentDir) setParentDir(s.defaultParentDir);
                                    loadCatalogs();
                                    setSettingsMsg("Imported and applied.");
                                }).catch((e: any) => setError(String(e)));
                            }}>Import…</button>
                        </div>
                        {settingsMsg && <pre className="output">{settingsMsg}</pre>}
                        {error && <pre className="error">{error}</pre>}
                        <section className="span2" id="sec-about">
                            <h3>About</h3>
                            <p>Templetry Desktop <strong>v{versions.app ?? "dev"}</strong>
                                {updates?.appUpdate && <> — <a onClick={() => OpenRepo(updates.appUrl)} className="upd">update {updates.appLatest} available</a></>}
                            </p>
                            <p>Embedded engine <strong>{versions.engine ?? "?"}</strong>
                                {updates?.engineUpdate && <> — <a onClick={() => OpenRepo(updates.engineUrl)} className="upd">engine {updates.engineLatest} released</a></>}
                            </p>
                            <div className="actions" style={{ marginTop: 14 }}>
                                <button onClick={() => checkUpdates(true)}>Check for updates</button>
                            </div>
                            {updMsg && <pre className="output">{updMsg}</pre>}
                        </section>
                        </div>
                    </>
                )}
                {view === "projects" && (
                    <>
                        <header><h2>My projects</h2></header>
                        <div className="outrow" style={{ marginTop: 16 }}>
                            <button onClick={loadProjects} disabled={busy}>Refresh</button>
                        </div>
                        {error && <pre className="error">{error}</pre>}
                        <div className="repolist">
                            {projects
                                .filter((p) => !projFilter || p.template === projFilter)
                                .map((p) => (
                                    <div key={p.dir} className="repo">
                                        <div className="repoinfo">
                                            <strong>
                                                {p.name}
                                                {drifts[p.dir] && (
                                                    <em className="driftchip"
                                                        title={`Template moved: ${(p.commit ?? "").slice(0, 7)} → ${drifts[p.dir].slice(0, 7)}`}>
                                                        template updated
                                                    </em>
                                                )}
                                            </strong>
                                            <span className="meta">{p.template} · {p.source}</span>
                                            <span className="desc">
                                                {Object.entries(p.variables ?? {}).map(([k, v]) => `${k}=${v}`).join(" · ")}
                                                {Object.entries(p.features ?? {}).filter(([, on]) => on).length > 0 &&
                                                    ` · features: ${Object.entries(p.features ?? {}).filter(([, on]) => on).map(([k]) => k).join(", ")}`}
                                            </span>
                                        </div>
                                        <div className="repoactions">
                                            {drifts[p.dir] && (
                                                <button className="primary" disabled={busy}
                                                    onClick={() => previewUpdate(p.dir)}>Preview update</button>
                                            )}
                                            <button onClick={() => OpenFolder(p.dir)}>Open folder</button>
                                        </div>
                                    </div>
                                ))}
                            {!projects.length && !busy && (
                                <div className="empty">No Templetry projects found in your repositories folder yet.</div>
                            )}
                        </div>
                        {updPrev && (
                            <section id="updpanel">
                                <h3>
                                    Update — {updPrev.template} · {(updPrev.oldCommit ?? "").slice(0, 7)} → {(updPrev.newCommit ?? "").slice(0, 7)}
                                    {" · "}{(updPrev.entries ?? []).length} changed · {updPrev.unchanged} unchanged
                                </h3>
                                {(updPrev.entries ?? []).length === 0 ? (
                                    <p className="hint">The template moved, but this project's output is identical — nothing to apply.</p>
                                ) : (
                                    <>
                                        <div className="preview" style={{ height: 380 }}>
                                            <div className="ptree">
                                                {(updPrev.entries ?? []).map((e: any) => (
                                                    <button key={e.path}
                                                        className={`pfile ${updSel === e.path ? "active" : ""}`}
                                                        onClick={() => {
                                                            setUpdSel(e.path);
                                                            UpdateFileContent(e.path).then((c: string) => setUpdContent(c))
                                                                .catch((err: any) => setUpdContent(String(err)));
                                                        }}>
                                                        <span>{e.path}</span>
                                                        <em>{e.status}</em>
                                                    </button>
                                                ))}
                                            </div>
                                            <pre className="pcontent">
                                                {updSel ? updContent : "Select a file to inspect its updated content."}
                                            </pre>
                                        </div>
                                        <div className="actions">
                                            <button className="primary" disabled={busy} onClick={applyUpdate}>
                                                Apply update
                                            </button>
                                            <button onClick={() => setUpdPrev(null)}>Dismiss</button>
                                        </div>
                                        <p className="hint">
                                            Apply writes added and modified files only — it never deletes yours. Review with git, then commit or discard.
                                        </p>
                                    </>
                                )}
                            </section>
                        )}
                        {repoMsg && <pre className="output">{repoMsg}</pre>}
                    </>
                )}
                {view === "repos" && (
                    <>
                        <header><h2>My repositories</h2></header>
                        <div className="outrow" style={{ marginTop: 16 }}>
                            <input placeholder="Search…" value={repoFilter}
                                onChange={(e) => setRepoFilter(e.target.value)} />
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
