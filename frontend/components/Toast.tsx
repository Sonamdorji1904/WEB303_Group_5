"use client";

import { createContext, useCallback, useContext, useState } from "react";
import { CheckCircle2, X, XCircle } from "lucide-react";

type ToastItem = { id: number; message: string; tone: "success" | "error" };
const ToastContext = createContext<(message: string, tone?: ToastItem["tone"]) => void>(() => undefined);

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const show = useCallback((message: string, tone: ToastItem["tone"] = "success") => {
    const id = Date.now() + Math.random();
    setItems((current) => [...current, { id, message, tone }]);
    window.setTimeout(() => setItems((current) => current.filter((item) => item.id !== id)), 4500);
  }, []);
  return <ToastContext.Provider value={show}>{children}<div className="fixed bottom-5 right-5 z-50 grid max-w-sm gap-3" aria-live="polite">{items.map((item) => <div key={item.id} className="flex items-start gap-3 rounded-xl bg-navy px-4 py-3 text-sm text-white shadow-xl"><span className={item.tone === "success" ? "text-green" : "text-red"}>{item.tone === "success" ? <CheckCircle2 size={18} /> : <XCircle size={18} />}</span><span className="flex-1">{item.message}</span><button onClick={() => setItems((current) => current.filter((x) => x.id !== item.id))} aria-label="Dismiss notification"><X size={16} /></button></div>)}</div></ToastContext.Provider>;
}
export const useToast = () => useContext(ToastContext);
