import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SPLIT_MAX, SPLIT_MIN, Splitter, clampWidth } from "./Splitter";

describe("clampWidth", () => {
    it("keeps a sensible width", () => {
        expect(clampWidth(320)).toBe(320);
    });

    // Dragging past either end must stop, not invert the layout.
    it("stops at both limits", () => {
        expect(clampWidth(10)).toBe(SPLIT_MIN);
        expect(clampWidth(5000)).toBe(SPLIT_MAX);
    });

    it("rounds, so the width never lands on a fraction of a pixel", () => {
        expect(clampWidth(320.6)).toBe(321);
    });

    // A stored value that is not a number would otherwise become NaN and
    // collapse the pane to nothing.
    it("survives a value that is not a number", () => {
        expect(clampWidth(NaN)).toBe(SPLIT_MIN);
    });
});

describe("Splitter", () => {
    let width = 300;
    const onChange = vi.fn((w: number) => { width = w; });

    beforeEach(() => { width = 300; onChange.mockClear(); });

    const setup = () => render(<Splitter width={width} onChange={onChange} />);

    it("announces itself as a separator with its current width", () => {
        setup();
        const sep = screen.getByRole("separator");
        expect(sep).toHaveAttribute("aria-valuenow", "300");
        expect(sep).toHaveAttribute("aria-orientation", "vertical");
    });

    // Draggable is not the same as usable.
    it("moves with the arrow keys", () => {
        setup();
        const sep = screen.getByRole("separator");
        fireEvent.keyDown(sep, { key: "ArrowRight" });
        expect(onChange).toHaveBeenLastCalledWith(316);
        fireEvent.keyDown(sep, { key: "ArrowLeft" });
        expect(onChange).toHaveBeenLastCalledWith(284);
    });

    it("moves further with shift held", () => {
        setup();
        fireEvent.keyDown(screen.getByRole("separator"), { key: "ArrowRight", shiftKey: true });
        expect(onChange).toHaveBeenLastCalledWith(348);
    });

    it("goes to either limit with Home and End", () => {
        setup();
        const sep = screen.getByRole("separator");
        fireEvent.keyDown(sep, { key: "Home" });
        expect(onChange).toHaveBeenLastCalledWith(SPLIT_MIN);
        fireEvent.keyDown(sep, { key: "End" });
        expect(onChange).toHaveBeenLastCalledWith(SPLIT_MAX);
    });

    it("reports the distance dragged, not the pointer position", () => {
        setup();
        const sep = screen.getByRole("separator");
        (sep as any).setPointerCapture = vi.fn();
        (sep as any).releasePointerCapture = vi.fn();

        fireEvent.pointerDown(sep, { clientX: 800, pointerId: 1 });
        fireEvent.pointerMove(sep, { clientX: 860, pointerId: 1 });
        // 300 + 60, not 860 — the handle knows nothing about where its
        // container begins, which is what lets it work in three layouts.
        expect(onChange).toHaveBeenLastCalledWith(360);
    });

    it("ignores movement when nothing is being dragged", () => {
        setup();
        fireEvent.pointerMove(screen.getByRole("separator"), { clientX: 900, pointerId: 1 });
        expect(onChange).not.toHaveBeenCalled();
    });
});
