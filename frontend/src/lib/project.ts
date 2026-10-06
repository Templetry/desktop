// repoKey identifies a repository across forges — the same owner/name can
// exist on GitHub and on a company GitLab. Mirrors repoKey() in repos.go.
export function repoKey(r: { forge?: string; fullName: string }): string {
    return ((r.forge ?? "") + "::" + r.fullName).toLowerCase();
}

// Drift covers both anchors a project carries: its template's commit and
// each applied piece's own (ADR-0016), so the chip has to say which moved.
export type Drift = { latest?: string; pieces?: string[] };

export function driftLabel(d: Drift): string {
    if (d.latest && d.pieces?.length) return "updates available";
    if (d.latest) return "template updated";
    return d.pieces?.length === 1 ? "piece updated" : "pieces updated";
}

export function driftTitle(p: { commit?: string }, d: Drift): string {
    const parts: string[] = [];
    if (d.latest) parts.push(`Template moved: ${(p.commit ?? "").slice(0, 7)} → ${d.latest.slice(0, 7)}`);
    if (d.pieces?.length) parts.push(`Pieces moved: ${d.pieces.join(", ")}`);
    return parts.join(" · ");
}

// resolveDoc joins a relative markdown link against the current doc's folder.
export function resolveDoc(from: string, href: string): string {
    const parts = from.includes("/") ? from.slice(0, from.lastIndexOf("/")).split("/") : [];
    for (const seg of href.split("#")[0].split("/")) {
        if (!seg || seg === ".") continue;
        if (seg === "..") parts.pop(); else parts.push(seg);
    }
    return parts.join("/");
}

// shortRemote is how a card names a remote: the host's own path, without the
// scheme or the .git suffix that make every GitHub URL start and end alike.
// The host stays unless it is github.com, where nearly every remote lives.
export function shortRemote(url: string): string {
    const u = url.trim().replace(/\.git$/, "").replace(/\/$/, "");
    const m = u.match(/^(?:[a-z+]+:\/\/)?(?:[^@/]+@)?([^/:]+)[/:](.+)$/i);
    if (!m) return u;
    const [, host, path] = m;
    return host.toLowerCase() === "github.com" ? path : `${host}/${path}`;
}

// patternProblem says why a value fails a manifest variable's pattern, or ""
// when it passes, is empty, or the pattern itself cannot be compiled — a
// broken pattern in a manifest is the engine's to report, not the form's.
export function patternProblem(value: string, pattern?: string): string {
    if (!value || !pattern) return "";
    let re: RegExp;
    try { re = new RegExp(pattern); } catch { return ""; }
    return re.test(value) ? "" : `Must match ${pattern}`;
}
