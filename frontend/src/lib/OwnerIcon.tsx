import { useState } from "react";

/**
 * An account's avatar, with a placeholder when there is none to show.
 *
 * The avatar URL is a guess built from the remote's host, so it is wrong for
 * some forges and unreachable for private ones. Rather than pretend, a failed
 * load falls back to a question mark: we know the repository belongs to
 * somebody, and we cannot see who.
 */
export function OwnerIcon({ src, owner, size = "sm" }: { src?: string; owner?: string; size?: "sm" | "md" }) {
    const [broken, setBroken] = useState(false);

    if (!src || broken) {
        return (
            <span className={`avatar ${size} unknown`} aria-hidden="true"
                title={owner ? `No avatar for ${owner}` : "No remote — this repository is local only"}>?</span>
        );
    }
    return (
        <img className={`avatar ${size}`} src={src} alt="" loading="lazy"
            onError={() => setBroken(true)} />
    );
}

/** The label for a group of projects with no identifiable account. */
export const UNKNOWN_OWNER = "::none";

/**
 * Groups projects by the account that owns them, in first-seen order so the
 * list does not reshuffle between scans. Projects with no remote collect
 * under one bucket at the end rather than disappearing.
 */
export function groupByOwner<T extends { owner?: string; avatarUrl?: string }>(items: T[]) {
    const groups: { owner: string; avatarUrl?: string; count: number }[] = [];
    const index = new Map<string, number>();

    for (const it of items) {
        const key = it.owner || UNKNOWN_OWNER;
        const at = index.get(key);
        if (at === undefined) {
            index.set(key, groups.length);
            groups.push({ owner: key, avatarUrl: it.avatarUrl, count: 1 });
        } else {
            groups[at].count++;
            if (!groups[at].avatarUrl) groups[at].avatarUrl = it.avatarUrl;
        }
    }

    // Unidentified last: it is a leftover, not a peer of the real accounts.
    return groups.sort((a, b) =>
        (a.owner === UNKNOWN_OWNER ? 1 : 0) - (b.owner === UNKNOWN_OWNER ? 1 : 0));
}
