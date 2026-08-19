import { render, screen } from "@testing-library/react";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { Tags } from "./Tags";

describe("Tags", () => {
    it("renders the kinds first, then languages and frameworks", () => {
        const { container } = render(
            <Tags of={{ kinds: ["frontend", "backend"], languages: ["typescript"], frameworks: ["nextjs"] }} />,
        );
        const chips = [...container.querySelectorAll(".taxtag")].map((e) => e.textContent);
        expect(chips).toEqual(["frontend", "backend", "typescript", "nextjs"]);
    });

    it("marks only the kinds as the primary axis", () => {
        const { container } = render(<Tags of={{ kinds: ["ios"], languages: ["swift"] }} />);
        expect([...container.querySelectorAll(".taxtag.kind")].map((e) => e.textContent)).toEqual(["ios"]);
    });

    // A form declaring nothing must render nothing, not an empty wrapper that
    // still takes a row's worth of margin.
    it("renders nothing at all when there is no taxonomy", () => {
        const { container } = render(<Tags of={{}} />);
        expect(container).toBeEmptyDOMElement();
    });

    it("keeps the caller's class alongside its own", () => {
        render(<Tags of={{ kinds: ["cli"] }} className="header" />);
        const wrapper = screen.getByText("cli").parentElement!;
        expect(wrapper).toHaveClass("taxtags", "header");
    });
});

// The wrapper class is not decoration: App.css excludes it from the rule that
// draws the arrow on the selected form. Without the exclusion the rule matched
// the chip row too and the arrow was drawn twice, on two lines — a bug that
// shipped and had to be reported by a user.
describe("the arrow rule it cooperates with", () => {
    const css = readFileSync(resolve(process.cwd(), "src/App.css"), "utf8");

    it("still excludes the chip row", () => {
        expect(css).toContain(".form.active > span:not(.taxtags)::before");
    });

    it("has no unscoped variant that would match the chips again", () => {
        expect(css).not.toMatch(/\.form\.active\s+span::before/);
    });

    it("puts the wrapper class where the rule expects it", () => {
        const { container } = render(<Tags of={{ kinds: ["go"] }} />);
        expect(container.firstElementChild?.tagName).toBe("SPAN");
        expect(container.firstElementChild).toHaveClass("taxtags");
    });
});
