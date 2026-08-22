import { describe, expect, it } from "vitest";
import { highlight, languageFor } from "./highlight";

describe("languageFor", () => {
    it("covers every ecosystem in the catalog", () => {
        const expected: Record<string, string> = {
            "main.go": "go",
            "src/main.rs": "rust",
            "app/settings.py": "python",
            "src/App.tsx": "typescript",
            "vite.config.js": "javascript",
            "build.gradle.kts": "kotlin",
            "Sources/App/AppConfig.swift": "swift",
            "src/Program.cs": "csharp",
            "Pages/Index.cshtml": "xml",
            "template.yml": "yaml",
            "package.json": "json",
            "Cargo.toml": "ini",
            "README.md": "markdown",
            "styles/app.scss": "scss",
            "schema.sql": "sql",
            "run.sh": "bash",
        };
        for (const [path, lang] of Object.entries(expected)) {
            expect(languageFor(path), path).toBe(lang);
        }
    });

    // A file whose name is the whole signal.
    it("knows files that carry no extension", () => {
        expect(languageFor("Dockerfile")).toBe("dockerfile");
        expect(languageFor("Makefile")).toBe("makefile");
        expect(languageFor(".gitignore")).toBe("properties");
    });

    // Every environment-profile file in the catalog looks like this.
    it("handles a suffixed name like .env.production", () => {
        expect(languageFor(".env.production")).toBe("properties");
        expect(languageFor("Dockerfile.dev")).toBe("dockerfile");
    });

    it("is case-insensitive and path-agnostic", () => {
        expect(languageFor("SRC/MAIN.GO")).toBe("go");
        expect(languageFor("a\windows\path\main.go")).toBe("go");
    });

    // Guessing wrong is worse than not guessing: the caller falls back to
    // automatic detection rather than mislabelling.
    it("declines what it does not know", () => {
        expect(languageFor("mystery.qqq")).toBeNull();
        expect(languageFor("noextension")).toBeNull();
    });
});

describe("highlight", () => {
    it("marks up a known language", () => {
        const html = highlight('package main\n\nfunc main() {}\n', "main.go");
        expect(html).toContain("hljs-keyword");
        expect(html).toContain("main");
    });

    // A template repository is not trusted input. What matters is that no
    // live element reaches the DOM — the characters may well appear, escaped,
    // because they are part of the file being shown.
    it("escapes markup instead of rendering it", () => {
        const html = highlight('const x = "<img src=x onerror=alert(1)>";', "a.ts");
        expect(html).not.toMatch(/<img/);
        expect(html).toContain("&lt;img");

        const el = document.createElement("div");
        el.innerHTML = html;
        expect(el.querySelector("img")).toBeNull();
    });

    // Auto-detection wraps pieces in spans, so compare the text it renders
    // rather than the markup around it.
    it("still returns the whole text for an unknown extension", () => {
        const el = document.createElement("div");
        el.innerHTML = highlight("just some words", "notes.qqq");
        expect(el.textContent).toBe("just some words");
    });
});
