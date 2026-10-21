export function StatusBadge({ status, pulse = false }: { status: string; pulse?: boolean }) {
  const styles: Record<string, string> = { PENDING: "bg-amber/10 text-amber", CONFIRMED: "bg-blue/10 text-blue", COMPLETED: "bg-green/10 text-green", REJECTED: "bg-red/10 text-red", AVAILABLE: "bg-green/10 text-green", HELD: "bg-amber/10 text-amber", RESERVED: "bg-blue/10 text-blue", SENT: "bg-green/10 text-green" };
  return <span className={`inline-flex items-center rounded-full px-2.5 py-1 text-xs font-bold ${styles[status] ?? "bg-slate-100 text-slate-600"} ${pulse ? "animate-pulse" : ""}`}>{status}</span>;
}
