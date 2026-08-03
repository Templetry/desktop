import { useEffect, useMemo, useState } from "react";
import {
    GetCatalog, GetTemplate, PlanProject, CreateProject, ChooseOutputDir,
    GetAuthStatus, StartGitHubLogin, Logout, PublishProject,
} from "../wailsjs/go/main/App";
import "./App.css";

type Form = { form: string; name: string; path: string; status: string; description?: string };
type Parent = { key: string; label?: string; repo: string; ref: string; forms: Form[] };
type Variable = { key: string; label?: string; type?: string; pattern?: string; options?: string[]; default?: string };
type Feature = { key: string; label?: string; default?: boolean };
type Manifest = { name: string; description?: string; variables?: Variable[]; features?: Feature[] };

function App() {
    const [parents, setParents] = useState<Parent[]>([]);
    const [selected, setSelected] = useState<string>("");
    const [manifest, setManifest] = useState<Manifest | null>(null);
    const [vars, setVars] = useState<Record<string, string>>({});
    const [feats, setFeats] = useState<Record<string, boolean>>({});
    const [outDir, setOutDir] = useState("");
    const [output, setOutput] = useState("");
    const [error, setError] = useState("");
    const [busy, setBusy] = useState(false);
    const [auth, setAuth] = useState<any>({ state: "logged_out" });
    const [createdDir, setCreatedDir] = useState("");
    const [repoName, setRepoName] = useState("");
    const [repoPrivate, setRepoPrivate] = useState(true);
    const [repoUrl, setRepoUrl] = useState("");

    useEffect(() => {
        GetCatalog()
            .then((reg: any) => setParents(reg.parents ?? []))
            .catch((e: any) => setError(String(e)));
        GetAuthStatus().then(setAuth).catch(() => {});
    }, []);

    useEffect(() => {
        if (auth.state !== "pending") return;
        const t = setInterval(() => GetAuthStatus().then(setAuth).catch(() => {}), 3000);
        return () => clearInterval(t);
    }, [auth.state]);

    const pick = async (ref: string) => {
        setSelected(ref);
        setManifest(null);
        setOutput("");
        setError("");
        setBusy(true);
        try {
            const m: Manifest = (await GetTemplate(ref)) as any;
            setManifest(m);
            const v: Record<string, string> = {};
            (m.variables ?? []).forEach((x) => { v[x.key] = x.default ?? ""; });
            setVars(v);
            const f: Record<string, boolean> = {};
            (m.features ?? []).forEach((x) => { f[x.key] = x.default ?? false; });
            setFeats(f);
        } catch (e) {
            setError(String(e));
        } finally {
            setBusy(false);
        }
    };

    const inputs = useMemo(() => {
        const v: Record<string, string> = {};
        Object.entries(vars).forEach(([k, val]) => { if (val !== "") v[k] = val; });
        return v;
    }, [vars]);

    const run = async (create: boolean) => {
        setBusy(true);
        setError("");
        setOutput("");
        try {
            const result = create
                ? await CreateProject(selected, outDir, inputs, feats)
                : await PlanProject(selected, inputs, feats);
            setOutput(String(result));
            if (create) {
                setCreatedDir(outDir);
                const seg = outDir.replace(/[\\/]+$/, "").split(/[\\/]/).pop() ?? "";
                setRepoName(seg.toLowerCase().replace(/[^a-z0-9._-]+/g, "-"));
                setRepoUrl("");
            }
        } catch (e) {
            setError(String(e));
        } finally {
            setBusy(false);
        }
    };

    const publish = async () => {
        setBusy(true);
        setError("");
        try {
            const url = await PublishProject(createdDir, repoName, `Created with Templetry (${selected})`, repoPrivate);
            setRepoUrl(String(url));
        } catch (e) {
            setError(String(e));
        } finally {
            setBusy(false);
        }
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
                                <button
                                    key={ref}
                                    className={`form ${selected === ref ? "active" : ""}`}
                                    disabled={!ready}
                                    onClick={() => pick(ref)}
                                    title={f.description}
                                >
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
                            <button onClick={() => Logout().then(() => setAuth({ state: "logged_out" }))}>
                                Sign out
                            </button>
                        </>
                    )}
                    {auth.state === "pending" && (
                        <div className="pending">
                            <span>Enter this code on GitHub:</span>
                            <strong>{auth.userCode}</strong>
                        </div>
                    )}
                    {(auth.state === "logged_out" || auth.state === "error") && (
                        <button
                            className="primary"
                            onClick={() => StartGitHubLogin().then(setAuth).catch((e: any) => setError(String(e)))}
                        >
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
                                <h3>Variables</h3>
                                {(manifest.variables ?? []).map((v) => (
                                    <label key={v.key} className="field">
                                        <span>{v.label ?? v.key}</span>
                                        {v.type === "select" ? (
                                            <select
                                                value={vars[v.key] ?? ""}
                                                onChange={(e) => setVars({ ...vars, [v.key]: e.target.value })}
                                            >
                                                {(v.options ?? []).map((o) => (
                                                    <option key={o} value={o}>{o}</option>
                                                ))}
                                            </select>
                                        ) : (
                                            <input
                                                value={vars[v.key] ?? ""}
                                                placeholder={v.pattern ?? ""}
                                                onChange={(e) => setVars({ ...vars, [v.key]: e.target.value })}
                                            />
                                        )}
                                    </label>
                                ))}
                            </section>
                        )}

                        {(manifest.features ?? []).length > 0 && (
                            <section>
                                <h3>Features</h3>
                                <div className="features">
                                    {(manifest.features ?? []).map((f) => (
                                        <label key={f.key} className="feature">
                                            <input
                                                type="checkbox"
                                                checked={feats[f.key] ?? false}
                                                onChange={(e) => setFeats({ ...feats, [f.key]: e.target.checked })}
                                            />
                                            <span>{f.label ?? f.key}</span>
                                        </label>
                                    ))}
                                </div>
                            </section>
                        )}

                        <section>
                            <h3>Output</h3>
                            <div className="outrow">
                                <input value={outDir} placeholder="Output directory…" readOnly />
                                <button onClick={async () => setOutDir((await ChooseOutputDir()) || outDir)}>
                                    Browse…
                                </button>
                            </div>
                        </section>

                        <div className="actions">
                            <button disabled={busy} onClick={() => run(false)}>Preview plan</button>
                            <button disabled={busy} className="primary" onClick={() => run(true)}>
                                Create project
                            </button>
                        </div>

                        {error && <pre className="error">{error}</pre>}
                        {output && <pre className="output">{output}</pre>}

                        {createdDir && !repoUrl && (
                            <section className="publish">
                                <h3>Publish to GitHub</h3>
                                {auth.state !== "logged_in" ? (
                                    <p className="hint">Sign in with GitHub (sidebar) to publish this project.</p>
                                ) : (
                                    <>
                                        <div className="outrow">
                                            <input
                                                value={repoName}
                                                onChange={(e) => setRepoName(e.target.value)}
                                                placeholder="repository-name"
                                            />
                                            <label className="feature">
                                                <input
                                                    type="checkbox"
                                                    checked={repoPrivate}
                                                    onChange={(e) => setRepoPrivate(e.target.checked)}
                                                />
                                                <span>Private</span>
                                            </label>
                                            <button className="primary" disabled={busy} onClick={publish}>
                                                Create repo & push
                                            </button>
                                        </div>
                                    </>
                                )}
                            </section>
                        )}
                        {repoUrl && (
                            <pre className="output">Published: {repoUrl}</pre>
                        )}
                    </>
                )}
                {selected && !manifest && !error && <div className="empty">Fetching template…</div>}
            </main>
        </div>
    );
}

export default App;
