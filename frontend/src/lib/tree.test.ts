import { describe, expect, it } from "vitest";
import { catKey, isExpanded, parentKey, toggle } from "./tree";

describe("tree keys", () => {
    // Two catalogs may both offer a "go" parent; the key has to keep them apart.
    it("scopes a parent to its catalog", () => {
        expect(parentKey("official", "go")).not.toBe(parentKey("mine", "go"));
    });

    it("cannot collide with a catalog key", () => {
        expect(catKey("go")).not.toBe(parentKey("go", "go"));
    });
});

describe("isExpanded", () => {
    it("is open until something closes it", () => {
        expect(isExpanded(new Set(), "c:official", false)).toBe(true);
    });

    it("is closed once collapsed", () => {
        expect(isExpanded(new Set(["c:official"]), "c:official", false)).toBe(false);
    });

    // A form that matches what you typed and stays hidden behind a closed
    // branch reads as "no results".
    it("ignores collapsed branches while a filter is running", () => {
        expect(isExpanded(new Set(["c:official"]), "c:official", true)).toBe(true);
    });
});

describe("toggle", () => {
    it("closes an open branch and reopens a closed one", () => {
        const once = toggle(new Set(), "c:x");
        expect(once.has("c:x")).toBe(true);
        expect(toggle(once, "c:x").has("c:x")).toBe(false);
    });

    // React compares by identity; mutating in place would not re-render.
    it("returns a new set rather than mutating", () => {
        const before = new Set<string>();
        expect(toggle(before, "c:x")).not.toBe(before);
        expect(before.size).toBe(0);
    });

    it("leaves other branches alone", () => {
        expect([...toggle(new Set(["a"]), "b")].sort()).toEqual(["a", "b"]);
    });
});
