// What the app may claim about updates.
//
// The engine is compiled into this binary, not invoked from PATH, so a newer
// engine release is not something the user can act on — it arrives with the
// next app build. Treating it as an available update sends people to a panel
// that tells them there is nothing to do.

export type Updates = {
    appUpdate?: boolean;
    engineUpdate?: boolean;
    appLatest?: string;
    engineLatest?: string;
    appUrl?: string;
    engineUrl?: string;
};

/**
 * Whether to offer the update affordance at all. Only an app update is
 * actionable: it has an installer behind it.
 */
export function isActionable(u: Updates | null | undefined): boolean {
    return !!u?.appUpdate;
}

/** The line shown after an explicit "check for updates". */
export function checkMessage(u: Updates | null | undefined): string {
    if (u?.appUpdate) return "Update available - see below.";
    if (u?.engineUpdate) {
        return "Up to date. A newer engine is out and arrives with the next app update.";
    }
    return "Everything is up to date.";
}

/**
 * The engine line in About. It is status, never an offer — the caller must
 * not render it as a link, because there is nothing to click.
 */
export function engineStatus(current: string | undefined, u: Updates | null | undefined): string {
    const running = current ?? "?";
    if (u?.engineUpdate && u.engineLatest) {
        return `${running} — ${u.engineLatest} is out and arrives with the next app update`;
    }
    return running;
}
