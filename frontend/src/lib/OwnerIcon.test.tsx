import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OwnerIcon, UNKNOWN_OWNER, groupByOwner } from "./OwnerIcon";

describe("groupByOwner", () => {
    const items = [
        { owner: "Templetry", avatarUrl: "a.png" },
        { owner: "Sebas1705", avatarUrl: "b.png" },
        { owner: "Templetry", avatarUrl: "a.png" },
        { owner: undefined },
    ];

    it("counts each account once", () => {
        const g = groupByOwner(items);
        expect(g.find((x) => x.owner === "Templetry")?.count).toBe(2);
        expect(g.find((x) => x.owner === "Sebas1705")?.count).toBe(1);
    });

    it("keeps repositories with no remote instead of dropping them", () => {
        expect(groupByOwner(items).find((x) => x.owner === UNKNOWN_OWNER)?.count).toBe(1);
    });

    // A list that reshuffles between scans is a list you cannot point at.
    it("puts the unidentified bucket last and leaves the rest in first-seen order", () => {
        expect(groupByOwner(items).map((x) => x.owner))
            .toEqual(["Templetry", "Sebas1705", UNKNOWN_OWNER]);
    });

    it("takes the avatar from whichever entry has one", () => {
        const g = groupByOwner([{ owner: "X" }, { owner: "X", avatarUrl: "late.png" }]);
        expect(g[0].avatarUrl).toBe("late.png");
    });
});

describe("OwnerIcon", () => {
    it("shows the avatar when there is one", () => {
        const { container } = render(<OwnerIcon src="https://github.com/Templetry.png" owner="Templetry" />);
        expect(container.querySelector("img")).toHaveAttribute("src", "https://github.com/Templetry.png");
    });

    // The avatar URL is a guess. When it is wrong, say so rather than
    // leaving a hole in the row.
    it("falls back to a question mark with no source", () => {
        render(<OwnerIcon owner="Private Org" />);
        expect(screen.getByText("?")).toBeInTheDocument();
    });

    it("explains the placeholder differently when there is no remote at all", () => {
        render(<OwnerIcon />);
        expect(screen.getByText("?")).toHaveAttribute("title", expect.stringContaining("local only"));
    });
});
