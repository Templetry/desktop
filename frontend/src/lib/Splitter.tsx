import { useCallback, useEffect, useRef, useState } from "react";

export const SPLIT_MIN = 150;
export const SPLIT_MAX = 720;

/** Keeps a pane width inside what is actually usable. */
export function clampWidth(w: number, min = SPLIT_MIN, max = SPLIT_MAX): number {
    if (!Number.isFinite(w)) return min;
    return Math.round(Math.min(max, Math.max(min, w)));
}

/**
 * A remembered pane width.
 *
 * Remembered per purpose, not globally: the file list of a rendered project
 * and the document list of a repository are read at different widths, and
 * one setting for both would keep losing whichever was adjusted last.
 */
export function useSplit(key: string, initial: number): [number, (w: number) => void] {
    const storageKey = "tpl.split." + key;
    const [width, setWidth] = useState(() => {
        const saved = Number(localStorage.getItem(storageKey));
        return saved ? clampWidth(saved) : clampWidth(initial);
    });
    const set = useCallback((w: number) => {
        const next = clampWidth(w);
        setWidth(next);
        localStorage.setItem(storageKey, String(next));
    }, [storageKey]);
    return [width, set];
}

/**
 * The drag handle between two panes.
 *
 * Tracks the distance moved rather than the pointer's absolute position, so
 * it needs to know nothing about where its container starts — which is what
 * lets the same handle sit in three different layouts.
 */
export function Splitter({ width, onChange, label = "Resize the list" }: {
    width: number;
    onChange: (w: number) => void;
    label?: string;
}) {
    const from = useRef<{ x: number; w: number } | null>(null);
    const [dragging, setDragging] = useState(false);

    // A drag that leaves the window must not leave the app in drag state.
    useEffect(() => {
        if (!dragging) return;
        const stop = () => { from.current = null; setDragging(false); };
        window.addEventListener("pointerup", stop);
        window.addEventListener("pointercancel", stop);
        return () => {
            window.removeEventListener("pointerup", stop);
            window.removeEventListener("pointercancel", stop);
        };
    }, [dragging]);

    return (
        <div
            className={`splitter ${dragging ? "dragging" : ""}`}
            role="separator"
            aria-orientation="vertical"
            aria-label={label}
            aria-valuenow={width}
            aria-valuemin={SPLIT_MIN}
            aria-valuemax={SPLIT_MAX}
            tabIndex={0}
            onPointerDown={(e) => {
                e.preventDefault();
                (e.currentTarget as Element).setPointerCapture(e.pointerId);
                from.current = { x: e.clientX, w: width };
                setDragging(true);
            }}
            onPointerMove={(e) => {
                if (!from.current) return;
                onChange(from.current.w + (e.clientX - from.current.x));
            }}
            onPointerUp={(e) => {
                from.current = null;
                setDragging(false);
                (e.currentTarget as Element).releasePointerCapture(e.pointerId);
            }}
            // Draggable is not the same as usable. Arrows move it, and Home
            // and End take it to either limit.
            onKeyDown={(e) => {
                const step = e.shiftKey ? 48 : 16;
                if (e.key === "ArrowLeft") { onChange(width - step); e.preventDefault(); }
                if (e.key === "ArrowRight") { onChange(width + step); e.preventDefault(); }
                if (e.key === "Home") { onChange(SPLIT_MIN); e.preventDefault(); }
                if (e.key === "End") { onChange(SPLIT_MAX); e.preventDefault(); }
            }}
        />
    );
}
