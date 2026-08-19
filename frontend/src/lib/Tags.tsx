import type { Taxonomy } from "./taxonomy";

/**
 * The three axes as chips. Kinds are marked so the primary axis reads first.
 *
 * The `taxtags` class on the wrapper is load-bearing: App.css uses it to
 * exclude these chips from the active-form arrow, which once rendered twice
 * because the rule matched every span in the row.
 */
export function Tags({ of, className = "" }: { of: Taxonomy; className?: string }) {
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
