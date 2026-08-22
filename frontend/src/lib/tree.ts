/** Keys for the catalog tree's collapsed set. */
export const catKey = (catalog: string) => "c:" + catalog;
export const parentKey = (catalog: string, parent: string) => "p:" + catalog + "/" + parent;

/**
 * Whether a branch shows its children.
 *
 * While a filter is running the collapsed set is ignored: a form that matches
 * what you typed and stays hidden behind a closed branch reads as "no
 * results", which is worse than no filter at all.
 */
export function isExpanded(collapsed: Set<string>, key: string, filtering: boolean): boolean {
    return filtering || !collapsed.has(key);
}

/** Toggling returns a new set, because React compares by identity. */
export function toggle(collapsed: Set<string>, key: string): Set<string> {
    const next = new Set(collapsed);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    return next;
}
