export type Identity = {
    login: string;
    avatar?: string;
    /** Where this identity lives, for the tooltip. */
    where: string;
};

type Auth = { state?: string; login?: string; avatar?: string };
type Account = { scheme?: string; host?: string; login?: string; avatar?: string };

/**
 * Everyone you are currently signed in as.
 *
 * GitHub comes first when present: its OAuth session is the app's own — it
 * carries the update check and every unauthenticated GitHub call — rather
 * than one account among several. The rest follow in the order they were
 * added.
 *
 * The GitHub *account* entry is skipped because the session above already
 * represents it; listing both would show the same person twice.
 */
export function signedInAs(auth: Auth | null | undefined, accounts: Account[] | null | undefined): Identity[] {
    const out: Identity[] = [];
    if (auth?.state === "logged_in" && auth.login) {
        out.push({ login: auth.login, avatar: auth.avatar, where: "github.com" });
    }
    for (const a of accounts ?? []) {
        if (a.scheme === "github" || !a.login) continue;
        out.push({ login: a.login, avatar: a.avatar, where: a.host ?? "" });
    }
    return out;
}

/**
 * What the button beside them should do.
 *
 * Signing out is a GitHub-session action; a forge account is removed in
 * Settings, one at a time, because there may be several. So the button
 * offers the only thing that makes sense for the state you are in — and
 * never claims you are signed out while you are signed in somewhere.
 */
export function sessionAction(auth: Auth | null | undefined, accounts: Account[] | null | undefined):
    "sign-out" | "accounts" | "sign-in" {
    if (auth?.state === "logged_in") return "sign-out";
    return signedInAs(auth, accounts).length > 0 ? "accounts" : "sign-in";
}
