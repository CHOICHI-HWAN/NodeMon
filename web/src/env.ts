// SPDX-License-Identifier: GPL-3.0-or-later
// VITE_DEMO=1 builds the demo: fake data, no Firebase, no login. Everything else talks to Firebase.
export const DEMO = import.meta.env.VITE_DEMO === "1";

declare const __APP_VERSION__: string; // set by vite.config.ts
export const APP_VERSION = __APP_VERSION__;
