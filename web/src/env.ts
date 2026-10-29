// Set VITE_DEMO=1 at build time to bundle fake data instead of calling the server.
export const DEMO = import.meta.env.VITE_DEMO === "1";
