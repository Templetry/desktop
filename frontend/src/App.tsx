import { useEffect, useMemo, useState } from "react";
import { marked } from "marked";
import DOMPurify from "dompurify";
import {
    GetCatalogs, GetTemplate, PreviewProject, PreviewFile, ChooseParentDir, GetLastParentDir,
    GetAuthStatus, StartGitHubLogin, Logout, GetOwners, CreateFullProject,
    ListRepos, OpenRepo, CloneRepo, GetSettings, SaveSettings, ExportSettings, ImportSettings,
    ScanProjects, OpenFolder, GetVersions, CheckUpdates, CheckDrift, CreateProjectOnRemote,
    GetAccounts, AddAccount, RemoveAccount, GetOwnerOptions, ListPieces, AddPiece,
    ListTemplateRepos, GetRepoOverview, GetRepoDoc, GetLocalOverview, GetLocalDoc,
    PreviewUpdate, UpdateFileContent, ApplyUpdate, InstallAppUpdate,
    GetVerifyInfo, StartVerify,
} from "../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../wailsjs/runtime";
import "./App.css";

// The taxonomy every form declares (ADR-0017). Three axes, each a list,
// because a form is usually more than one thing.
type Taxonomy = { kinds?: string[]; languages?: string[]; frameworks?: string[] };
type Form = { form: string; name: string; path: string; status: string; description?: string } & Taxonomy;
type TemplateForm = { path: string; name?: string; description?: string } & Taxonomy;

// The closed vocabulary, in the order the ADR lists it — not alphabetical,
// so the chips read as a spectrum rather than a dictionary.
const KINDS = [
    "frontend", "backend", "database", "infra",
    "multiplatform", "android", "ios", "desktop", "cli",
] as const;

function axesOf(t: Taxonomy): string[] {
    return [...(t.kinds ?? []), ...(t.languages ?? []), ...(t.frameworks ?? [])];
}

/** The three axes as chips. Kinds are marked so the primary axis reads first. */
function Tags({ of, className = "" }: { of: Taxonomy; className?: string }) {
    const kinds = of.kinds ?? [];
    const rest = [...(of.languages ?? []), ...(of.frameworks ?? [])];
    if (!kinds.length && !rest.length) return null;
    return (
        <span className={`taxtags ${className}`}>
            {kinds.map((k) => <em key={"k" + k} className="taxtag kind">{k}</em>)}
            {rest.map((v) => <em key={"v" + v} className="taxtag">{v}</em>)}
        </span>
    );
}
type Parent = { key: string; label?: string; repo: string; ref: string; forms: Form[] };
type Variable = { key: string; label?: string; type?: string; pattern?: string; options?: string[]; default?: string };
type Feature = { key: string; label?: string; default?: boolean; requires?: string[]; conflicts?: string[] };
type Preset = { key: string; label?: string; features?: Record<string, boolean> };
type Manifest = { name: string; description?: string; variables?: Variable[]; features?: Feature[]; presets?: Preset[] } & Taxonomy;

const LICENSES = ["", "mit", "apache-2.0", "gpl-3.0", "bsd-3-clause", "mpl-2.0", "unlicense"];

// Sentinel owner value for "bring your own remote" (ADR-0009 / ADR-0015):
// any git host, no forge API — the app only pushes.
const BYOR = "::byor";

// repoKey identifies a repository across forges — the same owner/name can
// exist on GitHub and on a company GitLab. Mirrors repoKey() in repos.go.
function repoKey(r: { forge?: string; fullName: string }) {
    return ((r.forge ?? "") + "::" + r.fullName).toLowerCase();
}

// Drift covers both anchors a project carries: its template's commit and
// each applied piece's own (ADR-0016), so the chip has to say which moved.
type Drift = { latest?: string; pieces?: string[] };

function driftLabel(d: Drift) {
    if (d.latest && d.pieces?.length) return "updates available";
    if (d.latest) return "template updated";
    return d.pieces?.length === 1 ? "piece updated" : "pieces updated";
}

function driftTitle(p: { commit?: string }, d: Drift) {
    const parts: string[] = [];
    if (d.latest) parts.push(`Template moved: ${(p.commit ?? "").slice(0, 7)} → ${d.latest.slice(0, 7)}`);
    if (d.pieces?.length) parts.push(`Pieces moved: ${d.pieces.join(", ")}`);
    return parts.join(" · ");
}

// resolveDoc joins a relative markdown link against the current doc's folder.
function resolveDoc(from: string, href: string) {
    const parts = from.includes("/") ? from.slice(0, from.lastIndexOf("/")).split("/") : [];
    for (const seg of href.split("#")[0].split("/")) {
        if (!seg || seg === ".") continue;
        if (seg === "..") parts.pop(); else parts.push(seg);
    }
    return parts.join("/");
}

// Markdown renders a doc as sanitized HTML; link clicks go through onLink.
function Markdown({ text, onLink }: { text: string; onLink: (href: string) => void }) {
    const html = useMemo(
        () => DOMPurify.sanitize(marked.parse(text, { async: false }) as string),
        [text],
    );
    return (
        <div className="pcontent md"
            onClick={(e) => {
                const a = (e.target as HTMLElement).closest("a");
                if (!a) return;
                e.preventDefault();
                onLink(a.getAttribute("href") ?? "");
            }}
            dangerouslySetInnerHTML={{ __html: html }} />
    );
}

function App() {
    const [catalogs, setCatalogs] = useState<any[]>([]);
    const [selectedCat, setSelectedCat] = useState("");
    const [selected, setSelected] = useState("");
    const [manifest, setManifest] = useState<Manifest | null>(null);
    const [vars, setVars] = useState<Record<string, string>>({});
    const [feats, setFeats] = useState<Record<string, boolean>>({});
    const [auth, setAuth] = useState<any>({ state: "logged_out" });
    const [owner, setOwner] = useState("");
    const [repoName, setRepoName] = useState("");
    const [repoPrivate, setRepoPrivate] = useState(true);
    const [remoteURL, setRemoteURL] = useState("");
    const [newAcc, setNewAcc] = useState({ scheme: "gitlab", host: "gitlab.com", token: "" });
    const [license, setLicense] = useState("");
    const [parentDir, setParentDir] = useState("");
    const [previewEntries, setPreviewEntries] = useState<any[]>([]);
    const [previewSel, setPreviewSel] = useState("");
    const [previewContent, setPreviewContent] = useState("");
    const [result, setResult] = useState<{ url: string; dir: string } | null>(null);
    const [error, setError] = useState("");
    const [busy, setBusy] = useState(false);
    const [view, setView] = useState<"build" | "cloud" | "settings" | "local">("build");
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

    // Catalog filtering (ADR-0017): kind chips are OR within the axis, and
    // the text box AND's over everything a form declares — the same
    // semantics as `templetry list --kind … --language …`.
    const [kindFilter, setKindFilter] = useState<string[]>([]);
    const [formFilter, setFormFilter] = useState("");

    const toggleKind = (k: string) =>
        setKindFilter((ks) => (ks.includes(k) ? ks.filter((x) => x !== k) : [...ks, k]));

    // Only the kinds the loaded catalogs actually use — a chip that can
    // never match anything is noise.
    const availableKinds = useMemo(() => {
        const present = new Set<string>();
        for (const c of catalogs)
            for (const p of c.parents ?? [])
                for (const f of p.forms ?? []) (f.kinds ?? []).forEach((k: string) => present.add(k));
        return KINDS.filter((k) => present.has(k));
    }, [catalogs]);

    const matchesFilter = (f: Form) => {
        if (kindFilter.length && !kindFilter.some((k) => (f.kinds ?? []).includes(k))) return false;
        const q = formFilter.trim().toLowerCase();
        if (!q) return true;
        return [f.form, f.name, f.description ?? "", ...axesOf(f)]
            .join(" ").toLowerCase().includes(q);
    };

    const filteringCatalog = kindFilter.length > 0 || formFilter.trim() !== "";

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

    const [drifts, setDrifts] = useState<Record<string, Drift>>({});
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

    const reposDirSet = !!(settings.defaultParentDir || settings.lastParentDir);

    // One-click fix for the "no repositories folder yet" callouts.
    const chooseReposFolder = async () => {
        const d = await ChooseParentDir();
        if (!d) return;
        const s = { ...settings, defaultParentDir: d };
        setSettings(s);
        setParentDir(d);
        await SaveSettings(s);
        loadProjects();
    };

    const loadProjects = () => {
        if (!reposDirSet) {
            setProjects([]);
            return;
        }
        setBusy(true);
        ScanProjects().then((p: any[]) => setProjects(p ?? []))
            .catch((e: any) => setError(String(e)))
            .finally(() => setBusy(false));
        CheckDrift().then((list: any[]) =>
            setDrifts(Object.fromEntries((list ?? []).map((d: any) => [d.dir, d]))))
            .catch(() => {});
    };

    const loadRepos = () => {
        setBusy(true);
        ListRepos().then((r: any[]) => setRepos(r ?? []))
            .catch((e: any) => setError(String(e)))
            .finally(() => setBusy(false));
        ListTemplateRepos()
            .then((names: string[]) => setTplRepos(Object.fromEntries((names ?? []).map((n) => [n, true]))))
            .catch(() => {});
        if (!projects.length) loadProjects();
    };

    // Cloud & Local previews — state summary panels.
    const [tplRepos, setTplRepos] = useState<Record<string, boolean>>({});
    const [cloudPrev, setCloudPrev] = useState<any>(null);
    const [cloudDoc, setCloudDoc] = useState("");
    const [cloudDocText, setCloudDocText] = useState("");
    const [localPrev, setLocalPrev] = useState<any>(null);
    const [localDoc, setLocalDoc] = useState("");
    const [localDocText, setLocalDocText] = useState("");

    const openCloudDoc = (fullName: string, p: string, forge: string) => {
        setCloudDoc(p);
        setCloudDocText("Loading…");
        GetRepoDoc(fullName, p, forge ?? "").then((t: string) => setCloudDocText(t))
            .catch((e: any) => setCloudDocText(String(e)));
    };

    const openCloudPreview = (r: any) => {
        setCloudPrev({ repo: r, data: null });
        setCloudDoc(""); setCloudDocText("");
        setTimeout(() => document.getElementById("cloudprev")?.scrollIntoView({ behavior: "smooth" }), 60);
        GetRepoOverview(r.fullName, r.forge ?? "").then((d: any) => {
            setCloudPrev({ repo: r, data: d });
            const readme = (d.docs ?? []).find((p: string) => p.toLowerCase() === "readme.md");
            if (readme) openCloudDoc(r.fullName, readme, r.forge ?? "");
        }).catch((e: any) => { setError(String(e)); setCloudPrev(null); });
    };

    const openLocalDoc = (dir: string, p: string) => {
        setLocalDoc(p);
        setLocalDocText("Loading…");
        GetLocalDoc(dir, p).then((t: string) => setLocalDocText(t))
            .catch((e: any) => setLocalDocText(String(e)));
    };

    // Pieces of the previewed project (ADR-0014).
    const [pieces, setPieces] = useState<any[]>([]);
    const [pieceVars, setPieceVars] = useState<Record<string, Record<string, string>>>({});

    const loadPieces = (p: any) => {
        setPieces([]);
        if (p.kind !== "templetry") return;
        ListPieces(p.dir)
            .then((list: any[]) => {
                setPieces(list ?? []);
                const seeds: Record<string, Record<string, string>> = {};
                (list ?? []).forEach((pc: any) => {
                    seeds[pc.name] = Object.fromEntries(
                        (pc.variables ?? []).map((v: any) => [v.key, v.default ?? ""]));
                });
                setPieceVars(seeds);
            })
            .catch(() => setPieces([]));
    };

    const adoptPiece = (dir: string, name: string) => {
        setBusy(true); setError("");
        AddPiece(dir, name, pieceVars[name] ?? {})
            .then((msg: string) => {
                setRepoMsg(msg);
                loadPieces(localPrev.proj);
                loadProjects();
            })
            .catch((e: any) => setError(String(e)))
            .finally(() => setBusy(false));
    };

    const openLocalPreview = (p: any) => {
        loadPieces(p);
        setLocalPrev({ proj: p, data: null });
        setLocalDoc(""); setLocalDocText("");
        setTimeout(() => document.getElementById("localprev")?.scrollIntoView({ behavior: "smooth" }), 60);
        GetLocalOverview(p.dir).then((d: any) => {
            setLocalPrev({ proj: p, data: d });
            const readme = (d.docs ?? []).find((x: string) => x.toLowerCase() === "readme.md");
            if (readme) openLocalDoc(p.dir, readme);
        }).catch((e: any) => { setError(String(e)); setLocalPrev(null); });
    };

    const ciGlyph = (r: any) => r.status !== "completed" ? "●" : r.conclusion === "success" ? "✓" : "✗";
    const ciClass = (r: any) => r.status !== "completed" ? "run" : r.conclusion === "success" ? "ok" : "bad";

    // Doc links: external ones open in the browser; relative .md ones
    // navigate inside the doc reader itself.
    const cloudLink = (href: string) => {
        if (/^https?:/i.test(href)) { OpenRepo(href); return; }
        const target = resolveDoc(cloudDoc, href);
        if (target.toLowerCase().endsWith(".md") && cloudPrev?.repo) {
            openCloudDoc(cloudPrev.repo.fullName, target, cloudPrev.repo.forge ?? "");
        }
    };
    const localLink = (href: string) => {
        if (/^https?:/i.test(href)) { OpenRepo(href); return; }
        const target = resolveDoc(localDoc, href);
        if (target.toLowerCase().endsWith(".md") && localPrev?.proj) {
            openLocalDoc(localPrev.proj.dir, target);
        }
    };

    // owner/name (lowercased) of a github remote URL, "" for anything else.
    const remoteFull = (u: string) => {
        const m = /github\.com[/:]([^/]+\/.+?)(\.git)?$/i.exec(u ?? "");
        return m ? m[1].toLowerCase() : "";
    };
    const localByRemote = useMemo(() => {
        const map: Record<string, string> = {};
        projects.forEach((p) => {
            const k = remoteFull(p.remote);
            if (k && !map[k]) map[k] = p.dir;
        });
        return map;
    }, [projects]);

    const applyUi = (s: any) => {
        const root = document.documentElement;
        root.dataset.theme = s?.uiTheme === "light" ? "light" : "dark";
        root.dataset.density = s?.uiDensity === "compact" ? "compact" : "comfortable";
        document.body.classList.toggle("layout-stacked", s?.uiLayout === "stacked");
        root.style.setProperty("--brass", s?.uiAccent || "#d9a441");
        (document.body.style as any).zoom = s?.uiScale || "1";
    };

    const switchView = (v: "build" | "cloud" | "settings" | "local") => {
        setView(v);
        setError("");
        setSettingsMsg("");
        setRepoMsg("");
        setUpdMsg("");
    };

    const login = () => StartGitHubLogin().then(setAuth).catch((e: any) => setError(String(e)));

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

    useEffect(() => {
        if (!updMsg) return;
        const t = setTimeout(() => setUpdMsg(""), 6000);
        return () => clearTimeout(t);
    }, [updMsg]);

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
        loadAccounts();
        if (auth.state === "logged_in" && view === "cloud" && !repos.length) loadRepos();
    }, [auth.state]);

    // Destinations across every signed-in forge (ADR-0015).
    const [accounts, setAccounts] = useState<any[]>([]);
    const [ownerOpts, setOwnerOpts] = useState<any[]>([]);
    const loadAccounts = () => {
        GetAccounts().then((a: any[]) => setAccounts(a ?? [])).catch(() => {});
        GetOwnerOptions().then((o: any[]) => {
            setOwnerOpts(o ?? []);
            if (!owner && o?.length) {
                const preferred = o.find((x: any) => x.key.endsWith("/" + settings.defaultOwner));
                setOwner(preferred ? preferred.key : o[0].key);
            }
        }).catch(() => {});
    };

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

    // The engine enforces requires/conflicts on the FINAL feature states and
    // refuses to auto-fix them, so the UI shows the problem rather than
    // silently ticking boxes — and blocks the actions that would just fail.
    const featureIssues = useMemo(() => {
        const issues: Record<string, string> = {};
        const labelOf = (k: string) =>
            (manifest?.features ?? []).find((f) => f.key === k)?.label ?? k;
        for (const f of manifest?.features ?? []) {
            if (!feats[f.key]) continue;
            for (const req of f.requires ?? []) {
                if (!feats[req]) issues[f.key] = `needs ${labelOf(req)}`;
            }
            for (const con of f.conflicts ?? []) {
                if (feats[con]) issues[f.key] = `cannot be combined with ${labelOf(con)}`;
            }
        }
        return issues;
    }, [manifest, feats]);

    const featureConflict = Object.keys(featureIssues).length > 0;

    const preview = async () => {
        setBusy(true); setError(""); setResult(null); setPreviewSel(""); setPreviewContent("");
        try {
            setPreviewEntries((await PreviewProject(selectedCat, selected, inputs, feats)) as any[]);
            setTimeout(() => document.querySelector(".previewsec")?.scrollIntoView({ behavior: "smooth", block: "start" }), 50);
        }
        catch (e) { setError(String(e)); } finally { setBusy(false); }
    };

    // Verify: render these inputs to a temp dir and build them in a
    // container. Output streams back as events, so a long build shows
    // progress instead of a frozen button.
    const [verifyInfo, setVerifyInfo] = useState<any>(null);
    const [verifying, setVerifying] = useState(false);
    const [verifyLog, setVerifyLog] = useState<string | null>(null);
    const [verifyOk, setVerifyOk] = useState<boolean | null>(null);

    useEffect(() => {
        EventsOn("verify:log", (chunk: string) => setVerifyLog((l) => (l ?? "") + chunk));
        EventsOn("verify:done", (d: any) => {
            setVerifying(false);
            setVerifyOk(!!d?.ok);
            if (!d?.ok && d?.error) setVerifyLog((l) => (l ?? "") + "\n" + d.error);
        });
        return () => { EventsOff("verify:log"); EventsOff("verify:done"); };
    }, []);

    // What a form can be verified with depends on the form, so ask whenever
    // the selection changes — and clear any log from the previous one.
    useEffect(() => {
        setVerifyLog(null); setVerifyOk(null); setVerifyInfo(null);
        if (!selected || !selectedCat) return;
        GetVerifyInfo(selectedCat, selected).then(setVerifyInfo).catch(() => setVerifyInfo(null));
    }, [selectedCat, selected]);

    const startVerify = async () => {
        setError(""); setVerifyLog(""); setVerifyOk(null); setVerifying(true);
        try {
            await StartVerify(selectedCat, selected, inputs, feats);
        } catch (e) {
            setVerifying(false);
            setVerifyOk(false);
            setError(String(e));
        }
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
            const r: any = owner === BYOR
                ? await CreateProjectOnRemote(selectedCat, selected, repoName, remoteURL, parentDir, inputs, feats)
                : await CreateFullProject(selectedCat, selected, owner, repoName, desc, license, repoPrivate, parentDir, inputs, feats);
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
                        <button className="primary" onClick={login}>
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
                        <button className={view === "build" ? "active" : ""} title="Build — new project"
                            onClick={() => switchView("build")}>+</button>
                        <button className={view === "cloud" ? "active" : ""} title="Cloud — GitHub repositories"
                            onClick={() => { switchView("cloud"); if (auth.state === "logged_in" && !repos.length) loadRepos(); }}>▤</button>
                        <button className={view === "local" ? "active" : ""} title="Local — repositories on disk"
                            onClick={() => { switchView("local"); loadProjects(); }}>▣</button>
                    </div>
                ) : (
                    <>
                        <div className="nav">
                            <button className={view === "build" ? "active" : ""} onClick={() => switchView("build")}>
                                Build
                            </button>
                            <button className={view === "cloud" ? "active" : ""}
                                onClick={() => { switchView("cloud"); if (auth.state === "logged_in" && !repos.length) loadRepos(); }}>
                                Cloud
                            </button>
                            <button className={view === "local" ? "active" : ""}
                                onClick={() => { switchView("local"); loadProjects(); }}>
                                Local
                            </button>
                        </div>
                        {view === "local" && (
                            <div className="parent">
                                <h2>Projects<em>{projects.length}</em></h2>
                                <button className={`form ${projFilter === "" ? "active" : ""}`}
                                    onClick={() => setProjFilter("")}>
                                    <span>All</span>
                                    <em>{projects.length}</em>
                                </button>
                                {[...new Set(projects.filter((p) => p.kind !== "git").map((p) => p.template))].map((t) => (
                                    <button key={t} className={`form ${projFilter === t ? "active" : ""}`}
                                        onClick={() => setProjFilter(t)}>
                                        <span>{t}</span>
                                        <em>{projects.filter((p) => p.template === t).length}</em>
                                    </button>
                                ))}
                                {projects.some((p) => p.kind === "git") && (
                                    <button className={`form ${projFilter === "::git" ? "active" : ""}`}
                                        onClick={() => setProjFilter("::git")}>
                                        <span>git repositories</span>
                                        <em>{projects.filter((p) => p.kind === "git").length}</em>
                                    </button>
                                )}
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
                        {view === "cloud" && (
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
                        {view === "build" && (
                            <div className="filters">
                                <input className="formsearch" value={formFilter} placeholder="Filter templates…"
                                    onChange={(e) => setFormFilter(e.target.value)} />
                                {availableKinds.length > 0 && (
                                    <div className="kindchips">
                                        {availableKinds.map((k) => (
                                            <button key={k}
                                                className={`taxtag kind ${kindFilter.includes(k) ? "on" : ""}`}
                                                title={`Show only ${k} templates`}
                                                onClick={() => toggleKind(k)}>{k}</button>
                                        ))}
                                    </div>
                                )}
                                {filteringCatalog && (
                                    <button className="clearfilter"
                                        onClick={() => { setKindFilter([]); setFormFilter(""); }}>
                                        Clear filter
                                    </button>
                                )}
                            </div>
                        )}
                        {view === "build" && catalogs.map((c) => {
                            const parents = (c.parents ?? [])
                                .map((p: Parent) => ({ p, forms: p.forms.filter(matchesFilter) }))
                                .filter((x: any) => x.forms.length > 0);
                            if (!parents.length && filteringCatalog && !c.error) return null;
                            return (
                                <div key={c.name} className="catalog">
                                    <div className="cathead">
                                        <span>{c.name}</span>
                                        {c.official && <em className="badge">official</em>}
                                    </div>
                                    {c.error && <p className="caterr">{c.error}</p>}
                                    {parents.map(({ p, forms }: { p: Parent; forms: Form[] }) => (
                                        <div key={c.name + p.key} className="parent">
                                            <h2>{p.label ?? p.key}<em>{forms.length}</em></h2>
                                            {forms.map((f) => {
                                                const ref = `${p.key}/${f.form}`;
                                                const active = selectedCat === c.name && selected === ref;
                                                const ready = !f.status || f.status === "ready";
                                                return (
                                                    <button key={ref} className={`form ${active ? "active" : ""}`}
                                                        disabled={!ready} onClick={() => pick(c.name, ref)} title={f.description}>
                                                        <span>{f.form}</span>
                                                        {!ready && <em>{f.status}</em>}
                                                        <Tags of={f} />
                                                    </button>
                                                );
                                            })}
                                        </div>
                                    ))}
                                </div>
                            );
                        })}
                        {view === "build" && filteringCatalog &&
                            !catalogs.some((c) => (c.parents ?? []).some((p: Parent) => p.forms.some(matchesFilter))) && (
                            <p className="caterr">
                                No template matches. Forms that declare no taxonomy never match a kind.
                            </p>
                        )}
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
                            {auth.state === "logged_in" && <p>Signed in as <strong>@{auth.login}</strong></p>}
                            {auth.state === "pending" && (
                                <p className="hint">Signing in — enter the code <strong>{auth.userCode}</strong> on GitHub to finish.</p>
                            )}
                            {auth.state !== "logged_in" && auth.state !== "pending" && (
                                <div className="callout">
                                    <span>Not signed in — sign in to create cloud repositories and browse yours.</span>
                                    <button className="primary" onClick={login}>Sign in with GitHub</button>
                                </div>
                            )}
                            <h3 style={{ marginTop: 22 }}>Accounts</h3>
                            {accounts.map((acc) => (
                                <div key={acc.scheme + "@" + acc.host} className="catrow">
                                    <span className="catname">
                                        {acc.avatar && <img className="avatar sm" alt="" src={acc.avatar} />}
                                        {acc.login} <em className="gitchip">{acc.scheme} · {acc.host}</em>
                                    </span>
                                    {acc.scheme === "github"
                                        ? <button onClick={() => Logout().then(() => { setAuth({ state: "logged_out" }); loadAccounts(); })}>Sign out</button>
                                        : <button onClick={() => {
                                            RemoveAccount(acc.scheme + "@" + acc.host);
                                            setSettingsMsg(`Removed ${acc.login} (${acc.host}).`);
                                            setTimeout(loadAccounts, 100);
                                        }}>Remove</button>}
                                </div>
                            ))}
                            <div className="catrow">
                                <select className="catnameinput" value={newAcc.scheme}
                                    onChange={(e) => setNewAcc({ ...newAcc, scheme: e.target.value, host: e.target.value === "gitlab" ? "gitlab.com" : "codeberg.org" })}>
                                    <option value="gitlab">GitLab</option>
                                    <option value="gitea">Gitea / Forgejo</option>
                                </select>
                                <input placeholder="host (gitlab.com, codeberg.org, git.mycompany.com…)" value={newAcc.host}
                                    onChange={(e) => setNewAcc({ ...newAcc, host: e.target.value })} />
                                <input type="password" placeholder="personal access token" value={newAcc.token}
                                    onChange={(e) => setNewAcc({ ...newAcc, token: e.target.value })} />
                                <button disabled={busy || !newAcc.host || !newAcc.token} onClick={() => {
                                    setBusy(true); setError("");
                                    AddAccount(newAcc.scheme, newAcc.host, newAcc.token)
                                        .then((acc: any) => {
                                            setSettingsMsg(`Signed in as ${acc.login} on ${acc.host}.`);
                                            setNewAcc({ scheme: "gitlab", host: "gitlab.com", token: "" });
                                            loadAccounts();
                                        })
                                        .catch((e: any) => setError(String(e)))
                                        .finally(() => setBusy(false));
                                }}>Add account</button>
                            </div>
                            <p className="hint">
                                GitHub signs in from the top bar (OAuth device flow). GitLab and Gitea/Forgejo use a
                                personal access token with <code>api</code> (GitLab) or <code>repo</code> (Gitea) scope —
                                the only method that works on self-hosted instances without registering an app. Tokens go
                                straight to your OS keyring, never into the settings file.
                            </p>
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
                                    {[...new Set(ownerOpts.map((o: any) => o.key.split("/").slice(1).join("/")))]
                                        .map((o) => <option key={o} value={o}>{o}</option>)}
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
                            {updates?.engineUpdate && (
                                <p className="hint">The engine ships inside the app — a newer engine arrives with the next app update, it cannot be updated separately.</p>
                            )}
                            <div className="actions" style={{ marginTop: 14 }}>
                                <button onClick={() => checkUpdates(true)}>Check for updates</button>
                                {updates?.appUpdate && (
                                    <button className="primary" disabled={busy} onClick={() => {
                                        setBusy(true);
                                        InstallAppUpdate()
                                            .then((tag: string) => setUpdMsg(`Installer for ${tag} launched - the app will close now.`))
                                            .catch((e: any) => setError(String(e)))
                                            .finally(() => setBusy(false));
                                    }}>Install {updates.appLatest}</button>
                                )}
                            </div>
                            {updMsg && <pre className="output">{updMsg}</pre>}
                        </section>
                        </div>
                    </>
                )}
                {view === "local" && (
                    <>
                        <header><h2>Local — repositories on disk</h2></header>
                        {!reposDirSet ? (
                            <div className="callout">
                                <span>My projects scans your repositories folder for Templetry projects and git repositories — choose that folder first.</span>
                                <button className="primary" onClick={chooseReposFolder}>Choose folder…</button>
                            </div>
                        ) : (
                        <>
                        <div className="outrow" style={{ marginTop: 16 }}>
                            <button onClick={loadProjects} disabled={busy}>Refresh</button>
                        </div>
                        {error && <pre className="error">{error}</pre>}
                        {Object.entries(projects
                            .filter((p) => !projFilter || (projFilter === "::git" ? p.kind === "git" : p.template === projFilter))
                            .reduce((g: Record<string, any[]>, p: any) => {
                                const rel = p.rel || p.name;
                                const folder = rel.includes("/") ? rel.slice(0, rel.lastIndexOf("/")) : "";
                                (g[folder] ??= []).push(p);
                                return g;
                            }, {}))
                            .sort(([a], [b]) => a.localeCompare(b))
                            .map(([folder, list]) => (
                            <div key={folder || "(root)"}>
                                <h3 className="folderhead">{folder || "·"}<em>{(list as any[]).length}</em></h3>
                                <div className="repolist" style={{ marginTop: 8 }}>
                                    {(list as any[]).map((p) => (
                                        <div key={p.dir} className="repo">
                                            <div className="repoinfo">
                                                <strong>
                                                    {p.name}
                                                    {p.kind === "git" && <em className="gitchip">git</em>}
                                                    {drifts[p.dir] && (
                                                        <em className="driftchip" title={driftTitle(p, drifts[p.dir])}>
                                                            {driftLabel(drifts[p.dir])}
                                                        </em>
                                                    )}
                                                </strong>
                                                <span className="meta">
                                                    {p.kind === "git"
                                                        ? [p.branch, p.remote || "local repository — no remote"].filter(Boolean).join(" · ")
                                                        : `${p.template} · ${p.source}`}
                                                </span>
                                                {p.kind !== "git" && (
                                                    <span className="desc">
                                                        {Object.entries(p.variables ?? {}).map(([k, v]) => `${k}=${v}`).join(" · ")}
                                                        {Object.entries(p.features ?? {}).filter(([, on]) => on).length > 0 &&
                                                            ` · features: ${Object.entries(p.features ?? {}).filter(([, on]) => on).map(([k]) => k).join(", ")}`}
                                                    </span>
                                                )}
                                            </div>
                                            <div className="repoactions">
                                                {drifts[p.dir] && (
                                                    <button className="primary" disabled={busy}
                                                        onClick={() => previewUpdate(p.dir)}>Preview update</button>
                                                )}
                                                <button title="Repo state: branches, remotes, docs"
                                                    onClick={() => openLocalPreview(p)}>▤ Preview</button>
                                                {(p.remote ?? "").startsWith("http") && (
                                                    <button title="Open origin in the browser"
                                                        onClick={() => OpenRepo(p.remote.replace(/\.git$/, ""))}>↗ Remote</button>
                                                )}
                                                <button title="Show in the file explorer"
                                                    onClick={() => OpenFolder(p.dir)}>⌂ Folder</button>
                                            </div>
                                        </div>
                                    ))}
                                </div>
                            </div>
                        ))}
                        {!projects.length && !busy && (
                            <div className="empty">Nothing here yet — no Templetry projects or git repositories found in your repositories folder.</div>
                        )}
                        {localPrev && (
                            <section id="localprev">
                                <h3>
                                    {localPrev.proj.rel || localPrev.proj.name}
                                    {localPrev.data?.branch && <em className="gitchip">{localPrev.data.branch}</em>}
                                </h3>
                                {!localPrev.data ? (
                                    <p className="hint">Loading overview…</p>
                                ) : (
                                    <>
                                        <div className="ovgrid">
                                            <p className="ovrow"><span>Branches</span>
                                                {(localPrev.data.branches ?? []).join(" · ") || "—"}
                                            </p>
                                            <p className="ovrow"><span>Remotes</span>
                                                {(localPrev.data.remotes ?? []).length
                                                    ? (localPrev.data.remotes ?? []).map((r: any) => `${r.name} → ${r.url}`).join(" · ")
                                                    : "none"}
                                            </p>
                                            {localPrev.data.lastCommit && (
                                                <p className="ovrow"><span>Last commit</span>{localPrev.data.lastCommit}</p>
                                            )}
                                            <p className="ovrow"><span>Working tree</span>
                                                {localPrev.data.changes < 0 ? "unknown"
                                                    : localPrev.data.changes === 0 ? "clean"
                                                    : `${localPrev.data.changes} uncommitted change${localPrev.data.changes === 1 ? "" : "s"}`}
                                            </p>
                                            {localPrev.proj.kind === "templetry" && (
                                                <p className="ovrow tplrow"><span>Template</span>
                                                    {localPrev.proj.template} · {localPrev.proj.source}
                                                </p>
                                            )}
                                        </div>
                                        {pieces.length > 0 && (
                                            <>
                                                <h3 style={{ marginTop: 20 }}>Pieces</h3>
                                                <p className="hint">
                                                    Decoupled units this project can adopt — from its own template and
                                                    from the shared catalogs. Adopting one adds only new files and its
                                                    declared patches; it never overwrites your work.
                                                </p>
                                                {pieces.map((pc: any) => (
                                                    <div key={pc.name} className="repo" style={{ marginTop: 8 }}>
                                                        <div className="repoinfo">
                                                            <strong>
                                                                {pc.name}
                                                                {pc.applied && <em className="driftchip">applied</em>}
                                                                {pc.common && (
                                                                    <em className="gitchip" title="Lives in a shared catalog repository, not in this template — fixed once, updated everywhere">
                                                                        common
                                                                    </em>
                                                                )}
                                                            </strong>
                                                            <span className="desc">{pc.description}</span>
                                                            {!pc.applied && (pc.variables ?? []).map((v: any) => (
                                                                <label key={v.key} className="field" style={{ marginTop: 6 }}>
                                                                    <span>{v.label ?? v.key}</span>
                                                                    <input
                                                                        value={pieceVars[pc.name]?.[v.key] ?? ""}
                                                                        placeholder={v.default ?? ""}
                                                                        onChange={(e) => setPieceVars({
                                                                            ...pieceVars,
                                                                            [pc.name]: { ...(pieceVars[pc.name] ?? {}), [v.key]: e.target.value },
                                                                        })} />
                                                                </label>
                                                            ))}
                                                        </div>
                                                        <div className="repoactions">
                                                            {pc.applied
                                                                ? <button disabled title="Already part of this project">✓ Applied</button>
                                                                : <button className="primary" disabled={busy}
                                                                    onClick={() => adoptPiece(localPrev.proj.dir, pc.name)}>
                                                                    + Add piece
                                                                </button>}
                                                        </div>
                                                    </div>
                                                ))}
                                            </>
                                        )}
                                        {(localPrev.data.docs ?? []).length > 0 && (
                                            <div className="preview" style={{ height: 320, marginTop: 12 }}>
                                                <div className="ptree">
                                                    {(localPrev.data.docs ?? []).map((p: string) => (
                                                        <button key={p} className={`pfile ${localDoc === p ? "active" : ""}`}
                                                            onClick={() => openLocalDoc(localPrev.proj.dir, p)}>
                                                            <span>{p}</span>
                                                        </button>
                                                    ))}
                                                </div>
                                                {localDoc
                                                    ? <Markdown text={localDocText} onLink={localLink} />
                                                    : <pre className="pcontent">Select a document to read it.</pre>}
                                            </div>
                                        )}
                                        <div className="actions">
                                            <button onClick={() => setLocalPrev(null)}>Dismiss</button>
                                        </div>
                                    </>
                                )}
                            </section>
                        )}
                        </>
                        )}
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
                {view === "cloud" && (
                    <>
                        <header><h2>Cloud — GitHub repositories</h2></header>
                        {auth.state !== "logged_in" ? (
                            <div className="callout">
                                <span>Your GitHub repositories appear here once you sign in.
                                    {auth.state === "pending" && <> Enter the code shown in the top bar to finish signing in.</>}
                                </span>
                                {auth.state !== "pending" && (
                                    <button className="primary" onClick={login}>Sign in with GitHub</button>
                                )}
                            </div>
                        ) : (
                        <>
                        {!reposDirSet && (
                            <div className="callout">
                                <span>Clone needs a repositories folder to clone into — choose it once and it sticks.</span>
                                <button className="primary" onClick={chooseReposFolder}>Choose folder…</button>
                            </div>
                        )}
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
                                            <strong>
                                                {r.fullName}
                                                {tplRepos[repoKey(r)] && (
                                                    <em className="driftchip" title="Contains a template.yml the engine can render">template</em>
                                                )}
                                                {localByRemote[r.fullName.toLowerCase()] && (
                                                    <em className="gitchip" title={localByRemote[r.fullName.toLowerCase()]}>cloned</em>
                                                )}
                                            </strong>
                                            <span className="meta">
                                                {r.private ? "private" : "public"}
                                                {r.language ? ` · ${r.language}` : ""}
                                                {r.archived ? " · archived" : ""}
                                                {` · ${String(r.updatedAt).slice(0, 10)}`}
                                            </span>
                                            {r.description && <span className="desc">{r.description}</span>}
                                        </div>
                                        <div className="repoactions">
                                            <button title="Repo state: branches, CI, docs"
                                                onClick={() => openCloudPreview(r)}>▤ Preview</button>
                                            <button title="Open on GitHub" onClick={() => OpenRepo(r.htmlUrl)}>↗ Open</button>
                                            {localByRemote[r.fullName.toLowerCase()] ? (
                                                <button title="Show the local clone in the file explorer"
                                                    onClick={() => OpenFolder(localByRemote[r.fullName.toLowerCase()])}>
                                                    ⌂ Folder
                                                </button>
                                            ) : (
                                                <button disabled={busy} title="Clone into your repositories folder" onClick={() => {
                                                    setBusy(true); setError(""); setRepoMsg("");
                                                    CloneRepo(r.cloneUrl, r.name, r.forge ?? "")
                                                        .then((d: string) => { setRepoMsg(`Cloned: ${d}`); loadProjects(); })
                                                        .catch((e: any) => setError(String(e)))
                                                        .finally(() => setBusy(false));
                                                }}>↓ Clone</button>
                                            )}
                                        </div>
                                    </div>
                                ))}
                            {!repos.length && !busy && <div className="empty">No repositories loaded.</div>}
                        </div>
                        {cloudPrev && (
                            <section id="cloudprev">
                                <h3>
                                    {cloudPrev.repo.fullName}
                                    {cloudPrev.data?.defaultBranch && <em className="gitchip">{cloudPrev.data.defaultBranch}</em>}
                                </h3>
                                {!cloudPrev.data ? (
                                    <p className="hint">Loading overview…</p>
                                ) : (
                                    <>
                                        {cloudPrev.data.description && <p className="ovdesc">{cloudPrev.data.description}</p>}
                                        <div className="ovgrid">
                                            {(cloudPrev.data.languages ?? []).length > 0 && (
                                                <p className="ovrow"><span>Languages</span>
                                                    {(cloudPrev.data.languages ?? []).map((l: any) => `${l.name} ${l.pct}%`).join(" · ")}
                                                </p>
                                            )}
                                            <p className="ovrow"><span>Branches</span>
                                                {(cloudPrev.data.branches ?? []).join(" · ") || "—"}
                                            </p>
                                            {(cloudPrev.data.templateForms ?? []).length > 0 && (
                                                <div className="ovrow tplrow">
                                                    <span>Template</span>
                                                    <div className="tplforms">
                                                        {(cloudPrev.data.templateForms ?? []).map((f: TemplateForm) => (
                                                            <div key={f.path} className="tplform">
                                                                <strong>{f.path === "." ? "(root)" : f.path}</strong>
                                                                {f.name && <em className="tplname">{f.name}</em>}
                                                                <Tags of={f} />
                                                                {f.description && <span className="desc">{f.description}</span>}
                                                                {!f.name && (
                                                                    <span className="desc">
                                                                        carries a template.yml the engine could not read
                                                                    </span>
                                                                )}
                                                            </div>
                                                        ))}
                                                    </div>
                                                </div>
                                            )}
                                        </div>
                                        {(cloudPrev.data.runs ?? []).length > 0 && (
                                            <div className="cirows">
                                                {(cloudPrev.data.runs ?? []).map((r: any, i: number) => (
                                                    <button key={i} className="cirow" title="Open the run on GitHub"
                                                        onClick={() => OpenRepo(r.url)}>
                                                        <em className={ciClass(r)}>{ciGlyph(r)}</em>
                                                        <span>{r.name}</span>
                                                        <span className="meta">{r.branch} · {String(r.updatedAt).slice(0, 10)}</span>
                                                    </button>
                                                ))}
                                            </div>
                                        )}
                                        {(cloudPrev.data.docs ?? []).length > 0 && (
                                            <div className="preview" style={{ height: 320, marginTop: 12 }}>
                                                <div className="ptree">
                                                    {(cloudPrev.data.docs ?? []).map((p: string) => (
                                                        <button key={p} className={`pfile ${cloudDoc === p ? "active" : ""}`}
                                                            onClick={() => openCloudDoc(cloudPrev.repo.fullName, p, cloudPrev.repo.forge ?? "")}>
                                                            <span>{p}</span>
                                                        </button>
                                                    ))}
                                                </div>
                                                {cloudDoc
                                                    ? <Markdown text={cloudDocText} onLink={cloudLink} />
                                                    : <pre className="pcontent">Select a document to read it.</pre>}
                                            </div>
                                        )}
                                        <div className="actions">
                                            <button onClick={() => setCloudPrev(null)}>Dismiss</button>
                                        </div>
                                    </>
                                )}
                            </section>
                        )}
                        </>
                        )}
                    </>
                )}
                {view === "build" && !selected && <div className="empty">Pick a template form to start.</div>}
                {view === "build" && selected && manifest && (
                    <>
                        <header>
                            <h2>{manifest.name}</h2>
                            {manifest.description && <p>{manifest.description}</p>}
                            <Tags of={manifest} className="header" />
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
                                {(manifest.presets ?? []).length > 0 && (
                                    <div className="field">
                                        <span>Preset</span>
                                        <div className="seg">
                                            {(manifest.presets ?? []).map((p) => (
                                                <button key={p.key} title="Apply this feature combo (you can still adjust below)"
                                                    onClick={() => {
                                                        const f: Record<string, boolean> = {};
                                                        (manifest.features ?? []).forEach((x) => { f[x.key] = x.default ?? false; });
                                                        Object.entries(p.features ?? {}).forEach(([k, v]) => { f[k] = v; });
                                                        setFeats(f);
                                                    }}>
                                                    {p.label ?? p.key}
                                                </button>
                                            ))}
                                        </div>
                                    </div>
                                )}
                                {(manifest.features ?? []).length > 0 && (
                                    <div className="features">
                                        {(manifest.features ?? []).map((f) => (
                                            <label key={f.key} className={`feature ${featureIssues[f.key] ? "bad" : ""}`}>
                                                <input type="checkbox" checked={feats[f.key] ?? false}
                                                    onChange={(e) => setFeats({ ...feats, [f.key]: e.target.checked })} />
                                                <span>{f.label ?? f.key}</span>
                                                {featureIssues[f.key] && (
                                                    <em className="fieldnote">{featureIssues[f.key]}</em>
                                                )}
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
                                    <option value="">(local only — no remote)</option>
                                    {ownerOpts.map((o) => <option key={o.key} value={o.key}>{o.label}</option>)}
                                    <option value={BYOR}>Any git host — paste a repository URL…</option>
                                </select>
                            </label>
                            {!ownerOpts.length && owner !== BYOR && (
                                <p className="hint">Sign in with GitHub (top bar) or add a GitLab/Gitea account (Settings → Accounts) to have the repo created for you — or pick “Any git host” to push to any server.</p>
                            )}
                            {owner === BYOR && (
                                <>
                                    <label className="field">
                                        <span>Repository URL</span>
                                        <input value={remoteURL} placeholder="https://gitlab.com/me/my-repo.git"
                                            onChange={(e) => setRemoteURL(e.target.value)} />
                                    </label>
                                    <p className="hint">
                                        Create an <strong>empty</strong> repository on your host, paste its clone URL, and Templetry
                                        renders, commits and pushes. Authentication uses your own git credentials (credential
                                        manager or SSH key) — the app never sees them.
                                    </p>
                                </>
                            )}
                            <label className="field">
                                <span>Name</span>
                                <input value={repoName} placeholder="my-new-repo"
                                    onChange={(e) => setRepoName(e.target.value.toLowerCase().replace(/[^a-z0-9._-]+/g, "-"))} />
                            </label>
                            {owner !== "" && owner !== BYOR && (
                                <>
                                    {owner.startsWith("github@") && (
                                        <label className="field">
                                            <span>License</span>
                                            <select value={license} onChange={(e) => setLicense(e.target.value)}>
                                                {LICENSES.map((l) => <option key={l} value={l}>{l === "" ? "(none)" : l}</option>)}
                                            </select>
                                        </label>
                                    )}
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
                            {!parentDir && <p className="hint">Required — Create stays disabled until you choose a folder.</p>}
                        </section>

                        <div className="actions">
                            <button disabled={busy || featureConflict} onClick={preview}>Preview</button>
                            <button disabled={busy || featureConflict || verifying || !verifyInfo?.available}
                                title={verifyInfo?.reason || `Render these inputs and build them in ${verifyInfo?.image}`}
                                onClick={startVerify}>
                                {verifying ? "Verifying…" : "Verify build"}
                            </button>
                            <button disabled={busy || featureConflict || !repoName || !parentDir || (owner === BYOR && !remoteURL)}
                                className="primary" onClick={create}>
                                {owner === BYOR ? "Create & push to remote" : owner ? "Create repo & project" : "Create project"}
                            </button>
                        </div>

                        {featureConflict && (
                            <p className="hint">
                                Resolve the feature combination above first — the engine refuses it rather than
                                guessing which side you meant.
                            </p>
                        )}
                        {verifyLog !== null && (
                            <section style={{ marginTop: 14 }}>
                                <h3>Verify {verifying ? "— running" : verifyOk === true ? "— passed" : verifyOk === false ? "— failed" : ""}</h3>
                                <p className="hint">
                                    The rendered project is built inside {verifyInfo?.image} (ADR-0004), so nothing
                                    is installed on this machine and nothing is written to your folders.
                                </p>
                                <pre className={verifyOk === false ? "error" : "output"}
                                    style={{ maxHeight: 320, overflow: "auto", whiteSpace: "pre-wrap" }}>
                                    {verifyLog || "Starting the container…"}
                                </pre>
                            </section>
                        )}

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
                {view === "build" && selected && !manifest && !error && <div className="empty">Fetching template…</div>}
            </main>
            </div>
        </div>
    );
}

export default App;
