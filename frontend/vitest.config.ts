import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

// Kept apart from vite.config.ts so the app build never carries the test
// toolchain, and so `npm run build` stays exactly what the release runs.
export default defineConfig({
    plugins: [react()],
    test: {
        environment: "jsdom",
        globals: true,
        setupFiles: ["./src/test/setup.ts"],
        // Only the modules that hold logic. App.tsx imports the Wails
        // bindings, which do not exist outside the running app.
        include: ["src/**/*.test.{ts,tsx}"],
    },
});
