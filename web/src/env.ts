// Build-time switches (see .env.* files):
//   VITE_DEMO=1                    fake data, no backend
//   VITE_BACKEND=firebase          Firebase Auth + Realtime Database (Spark plan, no server)
//   default                        the NodeMon Go server
export const BACKEND: "http" | "firebase" | "demo" =
  import.meta.env.VITE_DEMO === "1" ? "demo" : import.meta.env.VITE_BACKEND === "firebase" ? "firebase" : "http";
export const DEMO = BACKEND === "demo";
