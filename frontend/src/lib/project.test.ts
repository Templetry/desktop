import { describe, expect, it } from "vitest";
import { driftLabel, driftTitle, repoKey, resolveDoc } from "./project";

describe("repoKey", () => {
    // The bug this prevents: the same owner/name on two forges collapsing
    // into one row, so opening one opened the other.
    it("separates the same repository on different forges", () => {
        expect(repoKey({ forge: "github", fullName: "me/app" }))
            .not.toBe(repoKey({ forge: "gitlab", fullName: "me/app" }));
    });

    it("is case-insensitive, as the forges are", () => {
        expect(repoKey({ forge: "GitHub", fullName: "Me/App" }))
            .toBe(repoKey({ forge: "github", fullName: "me/app" }));
    });

    // Legacy rows recorded before multi-forge carry no forge at all.
    it("treats a missing forge as the empty one, not as a crash", () => {
        expect(repoKey({ fullName: "me/app" })).toBe("::me/app");
    });
});

describe("driftLabel", () => {
    it("names the template when only it moved", () => {
        expect(driftLabel({ latest: "abc1234" })).toBe("template updated");
    });

    it("says both when both moved", () => {
        expect(driftLabel({ latest: "abc1234", pieces: ["renovate"] })).toBe("updates available");
    });

    it("agrees in number with the pieces that moved", () => {
        expect(driftLabel({ pieces: ["renovate"] })).toBe("piece updated");
        expect(driftLabel({ pieces: ["renovate", "audit-trail"] })).toBe("pieces updated");
    });
});

describe("driftTitle", () => {
    it("abbreviates both commits to seven characters", () => {
        const title = driftTitle({ commit: "0123456789abcdef" }, { latest: "fedcba9876543210" });
        expect(title).toBe("Template moved: 0123456 → fedcba9");
    });

    it("lists the pieces that moved", () => {
        expect(driftTitle({ commit: "0123456789" }, { pieces: ["renovate", "audit-trail"] }))
            .toBe("Pieces moved: renovate, audit-trail");
    });

    it("joins both halves when both moved", () => {
        const title = driftTitle({ commit: "0123456789" }, { latest: "fedcba9876", pieces: ["renovate"] });
        expect(title).toBe("Template moved: 0123456 → fedcba9 · Pieces moved: renovate");
    });

    // A project whose answers file predates commit recording still has to
    // render a tooltip rather than throw on undefined.
    it("survives a project with no recorded commit", () => {
        expect(driftTitle({}, { latest: "fedcba9876" })).toBe("Template moved:  → fedcba9");
    });
});

describe("resolveDoc", () => {
    it("resolves a sibling against the current folder", () => {
        expect(resolveDoc("guide/getting-started.md", "authoring-templates.md"))
            .toBe("guide/authoring-templates.md");
    });

    it("walks up with ..", () => {
        expect(resolveDoc("guide/getting-started.md", "../adr/0017-template-taxonomy.md"))
            .toBe("adr/0017-template-taxonomy.md");
    });

    it("resolves against the root when the doc is at the root", () => {
        expect(resolveDoc("README.md", "state-of-the-art.md")).toBe("state-of-the-art.md");
    });

    it("drops the anchor", () => {
        expect(resolveDoc("README.md", "state-of-the-art.md#the-catalog")).toBe("state-of-the-art.md");
    });

    it("ignores . and empty segments", () => {
        expect(resolveDoc("guide/a.md", "./b.md")).toBe("guide/b.md");
        expect(resolveDoc("guide/a.md", "sub//b.md")).toBe("guide/sub/b.md");
    });
});
