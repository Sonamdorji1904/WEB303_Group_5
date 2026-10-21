"use client";

import { useEffect, useState } from "react";

export function usePolling<T>(load: () => Promise<T>, interval = 3000) {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    let timer: number | undefined;
    const run = async () => {
      if (document.hidden) return;
      try { const next = await load(); if (active) { setData(next); setError(""); } }
      catch (e) { if (active) { setData(null); setError(e instanceof Error ? e.message : "Unable to load data."); } }
      finally { if (active) setLoading(false); }
    };
    const schedule = () => { window.clearInterval(timer); timer = window.setInterval(run, interval); };
    run(); schedule();
    const onVisibility = () => { if (!document.hidden) run(); };
    document.addEventListener("visibilitychange", onVisibility);
    return () => { active = false; window.clearInterval(timer); document.removeEventListener("visibilitychange", onVisibility); };
  }, [load, interval]);
  return { data, loading, error };
}
