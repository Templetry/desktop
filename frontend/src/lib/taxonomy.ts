// The taxonomy every form declares (ADR-0017). Three axes, each a list,
// because a form is usually more than one thing.
export type Taxonomy = { kinds?: string[]; languages?: string[]; frameworks?: string[] };
export type Form = { form: string; name: string; path: string; status: string; description?: string } & Taxonomy;
export type TemplateForm = { path: string; name?: string; description?: string } & Taxonomy;

// The closed vocabulary, in the order the ADR lists it — not alphabetical,
// so the chips read as a spectrum rather than a dictionary.
export const KINDS = [
    "frontend", "backend", "database", "infra",
    "multiplatform", "android", "ios", "desktop", "cli",
] as const;

export function axesOf(t: Taxonomy): string[] {
    return [...(t.kinds ?? []), ...(t.languages ?? []), ...(t.frameworks ?? [])];
}

/**
 * matchesFilter decides whether a form survives the catalog filters: OR
 * within the kind axis, AND against the text query. A form with no kinds
 * matches no kind filter — deliberate, and the reason `meta/template` is
 * invisible while it declares none.
 */
export function matchesFilter(f: Form, kinds: readonly string[], query: string): boolean {
    if (kinds.length && !kinds.some((k) => (f.kinds ?? []).includes(k))) return false;
    const q = query.trim().toLowerCase();
    if (!q) return true;
    return [f.form, f.name, f.description ?? "", ...axesOf(f)]
        .join(" ").toLowerCase().includes(q);
}
