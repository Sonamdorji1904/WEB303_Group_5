"use client";

import { useEffect, useRef } from "react";
import { X } from "lucide-react";

export function Overlay({ title, onClose, children, wide = false }: { title: string; onClose: () => void; children: React.ReactNode; wide?: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    ref.current?.focus();
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
      if (event.key !== "Tab" || !ref.current) return;
      const focusable = ref.current.querySelectorAll<HTMLElement>("button, input, select, textarea, [href]");
      if (!focusable.length) return;
      const first = focusable[0], last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener("keydown", onKey);
    return () => { document.removeEventListener("keydown", onKey); previous?.focus(); };
  }, [onClose]);
  return <div className="fixed inset-0 z-40 flex justify-end bg-navy/30 p-0 backdrop-blur-sm" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
    <div ref={ref} tabIndex={-1} role="dialog" aria-modal="true" aria-label={title} className={`h-full w-full overflow-y-auto bg-white p-6 shadow-2xl sm:p-8 ${wide ? "max-w-2xl" : "max-w-lg"}`}>
      <div className="mb-6 flex items-center justify-between"><h2 className="text-xl font-bold text-navy">{title}</h2><button onClick={onClose} className="rounded-lg p-2 text-slate-500 hover:bg-canvas" aria-label="Close"><X size={20} /></button></div>
      {children}
    </div>
  </div>;
}

export function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: React.ReactNode }) {
  return <div className="fixed inset-0 z-40 flex items-center justify-center bg-navy/30 p-4 backdrop-blur-sm" onMouseDown={(event) => event.target === event.currentTarget && onClose()}><div className="w-full max-w-lg rounded-2xl bg-white p-6 shadow-2xl"><div className="mb-5 flex items-center justify-between"><h2 className="text-xl font-bold text-navy">{title}</h2><button onClick={onClose} className="rounded-lg p-2 text-slate-500 hover:bg-canvas" aria-label="Close"><X size={20} /></button></div>{children}</div></div>;
}
