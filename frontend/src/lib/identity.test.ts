import { describe, expect, it } from "vitest";
import { sessionAction, signedInAs } from "./identity";

const gh = { state: "logged_in", login: "sebss", avatar: "gh.png" };
const out = { state: "logged_out" };
const gitlab = { scheme: "gitlab", host: "gitlab.com", login: "sebas", avatar: "gl.png" };
const gitea = { scheme: "gitea", host: "codeberg.org", login: "seb" };

describe("signedInAs", () => {
    it("puts the GitHub session first — it is the app's own", () => {
        expect(signedInAs(gh, [gitlab]).map((i) => i.login)).toEqual(["sebss", "sebas"]);
    });

    // The bug this exists to fix: signed in to GitLab, and the top bar
    // offering to sign you in as though you were signed in nowhere.
    it("counts a forge account even with no GitHub session", () => {
        expect(signedInAs(out, [gitlab])).toHaveLength(1);
        expect(signedInAs(out, [gitlab])[0].where).toBe("gitlab.com");
    });

    // The session above already represents it; listing both shows the same
    // person twice.
    it("does not list the GitHub account beside the GitHub session", () => {
        const both = signedInAs(gh, [{ scheme: "github", host: "github.com", login: "sebss" }, gitlab]);
        expect(both.map((i) => i.login)).toEqual(["sebss", "sebas"]);
    });

    it("is empty when nothing is signed in", () => {
        expect(signedInAs(out, [])).toEqual([]);
        expect(signedInAs(null, null)).toEqual([]);
    });

    it("survives an account with no avatar", () => {
        expect(signedInAs(out, [gitea])[0].avatar).toBeUndefined();
    });
});

describe("sessionAction", () => {
    it("offers sign-out for the GitHub session", () => {
        expect(sessionAction(gh, [])).toBe("sign-out");
        expect(sessionAction(gh, [gitlab])).toBe("sign-out");
    });

    // Removing a forge account happens in Settings, one at a time, because
    // there may be several.
    it("sends you to Accounts when only forge accounts are signed in", () => {
        expect(sessionAction(out, [gitlab])).toBe("accounts");
    });

    it("offers sign-in only when nothing is signed in", () => {
        expect(sessionAction(out, [])).toBe("sign-in");
    });

    // Mid-flow there is a code on screen; offering the button again would
    // restart what is already running.
    it("does not offer sign-out while a sign-in is pending", () => {
        expect(sessionAction({ state: "pending" }, [])).toBe("sign-in");
    });
});
