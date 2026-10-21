"use client";

import { useCallback, useMemo, useState } from "react";
import { Bell, CalendarCheck, ClipboardCheck, FileHeart, Mail, UserPlus } from "lucide-react";
import { listNotifications, markNotificationRead } from "@/lib/api/notification";
import { usePolling } from "@/lib/hooks";
import { Notification } from "@/lib/types";
import { StatusBadge } from "@/components/StatusBadge";
import { useToast } from "@/components/Toast";
import { useAuth } from "@/components/AuthContext";

const icons: Record<string, typeof Bell> = { PatientRegistered: UserPlus, AppointmentRequested: CalendarCheck, AppointmentConfirmed: ClipboardCheck, AppointmentRejected: ClipboardCheck, AppointmentCompleted: FileHeart, MedicalRecordCreated: FileHeart };
const eventNames = ["PatientRegistered", "AppointmentRequested", "AppointmentConfirmed", "AppointmentRejected", "AppointmentCompleted", "MedicalRecordCreated"];
export default function NotificationsPage() {
  const { identity } = useAuth();
  const load = useCallback(() => listNotifications("", identity?.role === "ADMIN" ? "" : identity?.userId ?? ""), [identity?.role, identity?.userId]);
  const { data, loading, error } = usePolling(load);
  const [filter, setFilter] = useState("");
  const toast = useToast();
  const items = useMemo(() => (data ?? []).filter((item) => (identity?.role === "ADMIN" || item.recipientId === identity?.userId) && (!filter || item.eventType === filter)), [data, filter, identity?.role, identity?.userId]);
  async function read(item: Notification) { if (!item.readAt) { try { await markNotificationRead(item.id); toast("Notification marked as read."); } catch { toast("Unable to update notification.", "error"); } } }
  return <div className="space-y-7"><div className="flex flex-col justify-between gap-4 md:flex-row md:items-end"><div><p className="text-sm text-slate-500">Notification service</p><h2 className="mt-1 text-2xl font-bold text-navy">Event inbox</h2></div><label className="flex items-center gap-3"><span className="text-xs font-semibold uppercase tracking-wide text-slate-500">Filter</span><select className="input w-auto" value={filter} onChange={(e) => setFilter(e.target.value)}><option value="">All event types</option>{eventNames.map((name) => <option key={name}>{name}</option>)}</select></label></div>{error && <div className="rounded-lg border border-red/20 bg-red/5 px-4 py-3 text-sm text-red">Notification service is unavailable. Retrying automatically...</div>}<section className="card overflow-hidden">{loading && !data ? <div className="p-10 text-center text-sm text-slate-400">Loading notification inbox...</div> : items.length ? <div className="divide-y divide-line">{items.map((item) => <NotificationRow key={item.id} item={item} onRead={() => void read(item)} />)}</div> : <div className="p-12 text-center"><Bell className="mx-auto text-slate-300" size={30} /><p className="mt-3 text-sm text-slate-400">No notifications match this filter.</p></div>}</section></div>;
}
function NotificationRow({ item, onRead }: { item: Notification; onRead: () => void }) { const Icon = icons[item.eventType] ?? Bell; return <button onClick={onRead} className={`flex w-full items-start gap-4 px-5 py-5 text-left transition hover:bg-canvas sm:px-6 ${item.readAt ? "opacity-60" : ""}`}><div className="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-teal/10 text-teal"><Icon size={20} /></div><div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2"><span className="rounded-full bg-navy/5 px-2 py-1 text-[10px] font-bold uppercase tracking-wide text-navy">{item.eventType}</span><StatusBadge status={item.status} /><span className="ml-auto text-xs text-slate-400">{new Date(item.createdAt).toLocaleString()}</span></div><p className="mt-3 text-sm font-bold text-navy">{item.subject}</p><p className="mt-1 max-w-3xl text-sm leading-6 text-slate-500">{item.message}</p><p className="mt-3 flex items-center gap-2 text-xs text-slate-400"><Mail size={13} />{item.recipientName} · {item.recipientEmail || "No email"}</p></div></button>; }
