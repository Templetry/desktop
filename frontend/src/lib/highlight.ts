import hljs from "highlight.js/lib/common";
import dockerfile from "highlight.js/lib/languages/dockerfile";
import dos from "highlight.js/lib/languages/dos";
import gradle from "highlight.js/lib/languages/gradle";
import properties from "highlight.js/lib/languages/properties";
import scala from "highlight.js/lib/languages/scala";
import DOMPurify from "dompurify";

// The common bundle covers thirty-six languages; these five are ones a
// Templetry project actually contains and it does not.
hljs.registerLanguage("dockerfile", dockerfile);
hljs.registerLanguage("dos", dos);
hljs.registerLanguage("gradle", gradle);
hljs.registerLanguage("properties", properties);
hljs.registerLanguage("scala", scala);

/**
 * Extension to language. Everything here is a language highlight.js knows —
 * a name it does not know silently produces unstyled output, which looks
 * like a bug in the file rather than a gap in this table.
 */
const BY_EXTENSION: Record<string, string> = {
    // The twelve ecosystems in the catalog
    go: "go", mod: "ini", sum: "plaintext",
    rs: "rust", toml: "ini",
    py: "python", pyi: "python", cfg: "ini",
    ts: "typescript", tsx: "typescript", mts: "typescript", cts: "typescript",
    js: "javascript", jsx: "javascript", mjs: "javascript", cjs: "javascript",
    kt: "kotlin", kts: "kotlin", gradle: "gradle", java: "java",
    swift: "swift",
    cs: "csharp", csproj: "xml", sln: "plaintext", razor: "xml", cshtml: "xml",
    rb: "ruby", php: "php", scala: "scala", lua: "lua", pl: "perl", r: "r",
    c: "c", h: "c", cpp: "cpp", hpp: "cpp", cc: "cpp", m: "objectivec", mm: "objectivec",
    // Markup, config and data
    json: "json", jsonc: "json", json5: "json",
    yaml: "yaml", yml: "yaml",
    xml: "xml", html: "xml", htm: "xml", svg: "xml", plist: "xml", xaml: "xml",
    vue: "xml", svelte: "xml",
    css: "css", scss: "scss", sass: "scss", less: "less",
    md: "markdown", markdown: "markdown", mdc: "markdown", mdx: "markdown",
    sql: "sql", graphql: "graphql", gql: "graphql",
    sh: "bash", bash: "bash", zsh: "bash", ps1: "plaintext", bat: "dos", cmd: "dos",
    ini: "ini", conf: "ini", properties: "properties", env: "properties",
    diff: "diff", patch: "diff",
    txt: "plaintext", log: "plaintext",
};

/** Files whose whole name decides the language, extension or not. */
const BY_NAME: Record<string, string> = {
    dockerfile: "dockerfile",
    makefile: "makefile",
    gemfile: "ruby",
    rakefile: "ruby",
    ".gitignore": "properties",
    ".dockerignore": "properties",
    ".editorconfig": "ini",
    ".env": "properties",
    gradlew: "bash",
};

/**
 * The language to highlight a path as, or null when we should not guess.
 * Exported so the mapping can be tested without a DOM.
 */
export function languageFor(path: string): string | null {
    const file = path.split(/[\\/]/).pop() ?? "";
    const lower = file.toLowerCase();

    if (BY_NAME[lower]) return BY_NAME[lower];
    // Dockerfile.dev, .env.production — the meaningful part comes first.
    for (const name of Object.keys(BY_NAME)) {
        if (lower.startsWith(name + ".")) return BY_NAME[name];
    }

    const dot = lower.lastIndexOf(".");
    if (dot < 0) return null;
    const ext = lower.slice(dot + 1);
    return BY_EXTENSION[ext] ?? null;
}

/**
 * Highlighted HTML for a file's contents.
 *
 * highlight.js escapes what it emits, and the result is sanitized anyway:
 * this renders content that came from a template repository, and treating
 * that as trusted is exactly the assumption not worth making.
 */
export function highlight(code: string, path: string): string {
    const lang = languageFor(path);
    try {
        const out = lang && hljs.getLanguage(lang)
            ? hljs.highlight(code, { language: lang, ignoreIllegals: true })
            : hljs.highlightAuto(code);
        return DOMPurify.sanitize(out.value);
    } catch {
        // A highlighter that throws must not cost the user the file.
        return DOMPurify.sanitize(code.replace(/[<>&]/g, (c) =>
            ({ "<": "&lt;", ">": "&gt;", "&": "&amp;" }[c] as string)));
    }
}

/** Highlights a fenced block inside a markdown document. */
export function highlightFence(code: string, info: string): string {
    const lang = (info || "").trim().split(/\s+/)[0];
    try {
        if (lang && hljs.getLanguage(lang)) {
            return hljs.highlight(code, { language: lang, ignoreIllegals: true }).value;
        }
    } catch {
        /* fall through to the caller's own escaping */
    }
    return "";
}
