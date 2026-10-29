export type Theme = "auto" | "light" | "dark";

export function getTheme(): Theme {
  try {
    const v = localStorage.getItem("nm-theme");
    return v === "light" || v === "dark" ? v : "auto";
  } catch {
    return "auto";
  }
}

export function setTheme(t: Theme) {
  if (t === "auto") document.documentElement.removeAttribute("data-theme");
  else document.documentElement.setAttribute("data-theme", t);
  try {
    localStorage.setItem("nm-theme", t);
  } catch {
    /* storage unavailable: the choice just won't persist */
  }
}
