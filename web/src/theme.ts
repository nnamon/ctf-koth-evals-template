import { useEffect, useState } from "react";

export type Theme = "light" | "dark";

const STORAGE_KEY = "ctf-evals.theme";

function systemPrefersDark(): boolean {
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

function readStored(): Theme | null {
  const v = localStorage.getItem(STORAGE_KEY);
  return v === "light" || v === "dark" ? v : null;
}

function applyTheme(theme: Theme) {
  const cls = theme === "dark" ? "palette-m2d" : "palette-m2";
  document.body.classList.remove("palette-m2", "palette-m2d");
  document.body.classList.add(cls);
}

export function initTheme(): Theme {
  const initial: Theme = readStored() ?? (systemPrefersDark() ? "dark" : "light");
  applyTheme(initial);
  return initial;
}

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(() => readStored() ?? (systemPrefersDark() ? "dark" : "light"));

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  useEffect(() => {
    if (readStored() !== null) return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = (e: MediaQueryListEvent) => setTheme(e.matches ? "dark" : "light");
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  const setAndPersist = (next: Theme) => {
    localStorage.setItem(STORAGE_KEY, next);
    setTheme(next);
  };

  const toggle = () => setAndPersist(theme === "dark" ? "light" : "dark");

  return { theme, setTheme: setAndPersist, toggle };
}
