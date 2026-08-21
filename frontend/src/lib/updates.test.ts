import { describe, expect, it } from "vitest";
import { checkMessage, engineStatus, isActionable, type Updates } from "./updates";

const engineOnly: Updates = { appUpdate: false, engineUpdate: true, engineLatest: "1.10.1" };
const appOnly: Updates = { appUpdate: true, engineUpdate: false, appLatest: "1.9.0" };
const both: Updates = { appUpdate: true, engineUpdate: true, appLatest: "1.9.0", engineLatest: "1.10.1" };
const none: Updates = { appUpdate: false, engineUpdate: false };

describe("isActionable", () => {
    // The bug this exists to prevent: the app announced "Update available",
    // sent you to the About panel, and told you there was nothing to do.
    it("does not offer an update when only the engine moved", () => {
        expect(isActionable(engineOnly)).toBe(false);
    });

    it("offers one when the app moved", () => {
        expect(isActionable(appOnly)).toBe(true);
        expect(isActionable(both)).toBe(true);
    });

    it("offers nothing when nothing moved, or before the check has run", () => {
        expect(isActionable(none)).toBe(false);
        expect(isActionable(null)).toBe(false);
        expect(isActionable(undefined)).toBe(false);
    });
});

describe("checkMessage", () => {
    it("says up to date when only the engine is ahead — because the user is", () => {
        expect(checkMessage(engineOnly)).toContain("Up to date");
    });

    it("explains where the newer engine will come from", () => {
        expect(checkMessage(engineOnly)).toContain("next app update");
    });

    it("points at the installer when there is one", () => {
        expect(checkMessage(appOnly)).toBe("Update available - see below.");
        expect(checkMessage(both)).toBe("Update available - see below.");
    });

    it("is unambiguous when everything is current", () => {
        expect(checkMessage(none)).toBe("Everything is up to date.");
    });
});

describe("engineStatus", () => {
    it("is the bare version when nothing is newer", () => {
        expect(engineStatus("1.10.1", none)).toBe("1.10.1");
    });

    it("names the newer one and where it arrives from", () => {
        expect(engineStatus("1.10.0", engineOnly))
            .toBe("1.10.0 — 1.10.1 is out and arrives with the next app update");
    });

    // Build info can fail to report the module version.
    it("survives an unknown running version", () => {
        expect(engineStatus(undefined, none)).toBe("?");
    });
});
