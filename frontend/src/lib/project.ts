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
