import { useEffect, useMemo, useState } from "react";
import {
    GetCatalog, GetTemplate, PlanProject, ChooseParentDir, GetLastParentDir,
    GetAuthStatus, StartGitHubLogin, Logout, GetOwners, CreateFullProject,
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
    const [output, setOutput] = useState("");
    const [result, setResult] = useState<{ url: string; dir: string } | null>(null);
    const [error, setError] = useState("");
    const [busy, setBusy] = useState(false);

    useEffect(() => {
        GetCatalog().then((r: any) => setParents(r.parents ?? [])).catch((e: any) => setError(String(e)));
        GetAuthStatus().then(setAuth).catch(() => {});
        GetLastParentDir().then((d: string) => d && setParentDir(d)).catch(() => {});
    }, []);

    useEffect(() => {
        if (auth.state !== "pending") return;
        const t = setInterval(() => GetAuthStatus().then(setAuth).catch(() => {}), 3000);
        return () => clearInterval(t);
    }, [auth.state]);

    useEffect(() => {
        if (auth.state === "logged_in") {
            GetOwners().then((o: string[]) => { setOwners(o); if (!owner) setOwner(o[0] ?? ""); }).catch(() => {});
        } else {
            setOwners([]);
            setOwner("");
        }
    }, [auth.state]);

    const pick = async (ref: string) => {
        setSelected(ref); setManifest(null); setOutput(""); setResult(null); setError(""); setBusy(true);
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
        setBusy(true); setError(""); setOutput(""); setResult(null);
        try { setOutput(String(await PlanProject(selected, inputs, feats))); }
        catch (e) { setError(String(e)); } finally { setBusy(false); }
    };

    const create = async () => {
        setBusy(true); setError(""); setOutput(""); setResult(null);
        try {
            const desc = `Created with Templetry (${selected})`;
            const r: any = await CreateFullProject(selected, owner, repoName, desc, license, repoPrivate, parentDir, inputs, feats);
            setResult(r);
        } catch (e) { setError(String(e)); } finally { setBusy(false); }
    };

    return (
        <div id="app">
            <aside>
                <h1>Templetry</h1>
                <p className="tag">Project scaffolding for every platform</p>
                {parents.map((p) => (
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
                <div className="authbox">
                    {auth.state === "logged_in" && (
                        <>
                            <span className="who">@{auth.login}</span>
                            <button onClick={() => Logout().then(() => setAuth({ state: "logged_out" }))}>Sign out</button>
                        </>
                    )}
                    {auth.state === "pending" && (
                        <div className="pending"><span>Enter this code on GitHub:</span><strong>{auth.userCode}</strong></div>
                    )}
                    {(auth.state === "logged_out" || auth.state === "error") && (
                        <button className="primary"
                            onClick={() => StartGitHubLogin().then(setAuth).catch((e: any) => setError(String(e)))}>
                            Sign in with GitHub
                        </button>
                    )}
                    {auth.error && <span className="autherr">{auth.error}</span>}
                </div>
            </aside>

            <main>
                {!selected && <div className="empty">Pick a template form to start.</div>}
                {selected && manifest && (
                    <>
                        <header>
                            <h2>{manifest.name}</h2>
                            {manifest.description && <p>{manifest.description}</p>}
                        </header>

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
                            <button disabled={busy} onClick={preview}>Preview plan</button>
                            <button disabled={busy || !repoName || !parentDir} className="primary" onClick={create}>
                                {owner ? "Create repo & project" : "Create project"}
                            </button>
                        </div>

                        {busy && <p className="hint">Working…</p>}
                        {error && <pre className="error">{error}</pre>}
                        {output && <pre className="output">{output}</pre>}
                        {result && (
                            <pre className="output">
                                {result.url ? `Repository: ${result.url}\n` : ""}Local: {result.dir}
                            </pre>
                        )}
                    </>
                )}
                {selected && !manifest && !error && <div className="empty">Fetching template…</div>}
            </main>
        </div>
    );
}

export default App;
