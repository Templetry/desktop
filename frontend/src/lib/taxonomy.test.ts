import { describe, expect, it } from "vitest";
import { KINDS, axesOf, matchesFilter, type Form } from "./taxonomy";

const form = (over: Partial<Form> = {}): Form => ({
    form: "react-spa", name: "web-react-spa", path: "react-spa", status: "ready",
    description: "React single-page app",
    kinds: ["frontend"], languages: ["typescript"], frameworks: ["react", "vite"],
    ...over,
});

describe("KINDS", () => {
    it("is the closed vocabulary of ADR-0017, in the ADR's order", () => {
        expect([...KINDS]).toEqual([
            "frontend", "backend", "database", "infra",
            "multiplatform", "android", "ios", "desktop", "cli",
        ]);
    });
});

describe("axesOf", () => {
    it("concatenates the three axes", () => {
        expect(axesOf(form())).toEqual(["frontend", "typescript", "react", "vite"]);
    });

    it("tolerates a form declaring none of them", () => {
        expect(axesOf({})).toEqual([]);
    });
});

describe("matchesFilter", () => {
    it("keeps everything when nothing is filtered", () => {
        expect(matchesFilter(form(), [], "")).toBe(true);
    });

    it("ORs within the kind axis", () => {
        const f = form({ kinds: ["frontend", "backend"] });
        expect(matchesFilter(f, ["backend"], "")).toBe(true);
        expect(matchesFilter(f, ["cli", "backend"], "")).toBe(true);
        expect(matchesFilter(f, ["cli"], "")).toBe(false);
    });

    // The behaviour that makes meta/template invisible today. If this ever
    // becomes "untagged matches everything", that is a product decision and
    // this test should be the thing that objects.
    it("excludes an untagged form from every kind filter", () => {
        const untagged = form({ kinds: undefined, languages: undefined, frameworks: undefined });
        expect(matchesFilter(untagged, ["frontend"], "")).toBe(false);
        expect(matchesFilter(untagged, [], "")).toBe(true);
    });

    it("searches the name, the form key and the description", () => {
        expect(matchesFilter(form(), [], "react-spa")).toBe(true);
        expect(matchesFilter(form(), [], "single-page")).toBe(true);
        expect(matchesFilter(form(), [], "svelte")).toBe(false);
    });

    // The reason the query looks at axesOf: typing a framework name is the
    // obvious way to look for it, and it appears in no other field.
    it("searches the taxonomy too", () => {
        expect(matchesFilter(form(), [], "vite")).toBe(true);
        expect(matchesFilter(form(), [], "typescript")).toBe(true);
    });

    it("ignores case and surrounding space", () => {
        expect(matchesFilter(form(), [], "  REACT  ")).toBe(true);
    });

    it("ANDs the two filters", () => {
        expect(matchesFilter(form(), ["frontend"], "react")).toBe(true);
        expect(matchesFilter(form(), ["backend"], "react")).toBe(false);
        expect(matchesFilter(form(), ["frontend"], "svelte")).toBe(false);
    });

    it("survives a form with no description", () => {
        expect(matchesFilter(form({ description: undefined }), [], "react")).toBe(true);
    });
});
