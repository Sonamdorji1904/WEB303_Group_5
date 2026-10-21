"use client";

import { FormEvent, useCallback, useState } from "react";
import { Check, ClipboardList, X } from "lucide-react";
import { acceptRequest, listRequests, rejectRequest } from "@/lib/api/doctor";
import { usePolling } from "@/lib/hooks";
import { AppointmentRequest } from "@/lib/types";
import { Modal } from "@/components/Overlay";
import { StatusBadge } from "@/components/StatusBadge";
import { useToast } from "@/components/Toast";
import { useAuth } from "@/components/AuthContext";

export default function RequestsPage() {
  const { identity } = useAuth();
  const load = useCallback(() => listRequests("PENDING", identity?.userId), [identity?.userId]);
  const { data: requests, loading, error } = usePolling(load);
  const ownRequests = (requests ?? []).filter((item) => !identity?.userId || item.doctorId === identity.userId);
  const [target, setTarget] = useState<AppointmentRequest | null>(null);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const toast = useToast();

  async function accept(item: AppointmentRequest) {
    setBusy(true);
    try { await acceptRequest(item.id); toast("Request accepted. The appointment is now confirmed."); }
    catch (e) { toast(e instanceof Error ? e.message : "Unable to accept request.", "error"); }
    finally { setBusy(false); }
  }
  async function reject(event: FormEvent) {
    event.preventDefault();
    if (!target || !reason.trim()) return;
    setBusy(true);
    try { await rejectRequest(target.id, reason.trim()); setTarget(null); setReason(""); toast("Request rejected and the slot was released."); }
    catch (e) { toast(e instanceof Error ? e.message : "Unable to reject request.", "error"); }
    finally { setBusy(false); }
  }

  return <div className="space-y-7">
    <div><p className="text-sm text-slate-500">Doctor service</p><h2 className="mt-1 text-2xl font-bold text-navy">Doctor requests</h2><p className="mt-2 text-sm text-slate-500">Review appointments waiting for approval. Held slots remain unavailable until you decide.</p></div>
    <section className="card overflow-hidden">
      <div className="flex items-center justify-between border-b border-line px-6 py-5"><div className="flex items-center gap-3"><div className="grid h-10 w-10 place-items-center rounded-xl bg-teal/10 text-teal"><ClipboardList size={20} /></div><div><h3 className="font-bold text-navy">Pending requests</h3><p className="text-xs text-slate-400">Polling every 3 seconds</p></div></div><span className="rounded-full bg-amber/10 px-3 py-1 text-xs font-bold text-amber">{ownRequests.length} pending</span></div>
      {loading && !requests ? <div className="p-10 text-center text-sm text-slate-400">Loading requests...</div> : error ? <div className="p-10 text-center text-sm text-red">Doctor service is unavailable. Retrying automatically...</div> : ownRequests.length ? <div className="divide-y divide-line">{ownRequests.map((item) => <div key={item.id} className="flex flex-col gap-4 px-6 py-5 lg:flex-row lg:items-center lg:justify-between"><div><div className="flex flex-wrap items-center gap-2"><h4 className="font-bold text-navy">{item.patientName}</h4><StatusBadge status={item.status} pulse /></div><p className="mt-1 text-sm text-slate-500">{item.date} · {item.time} · {item.reason}</p><p className="mt-1 text-xs text-slate-400">Appointment {item.appointmentId}</p></div><div className="flex gap-2"><button className="btn-primary px-3 py-2 text-sm" disabled={busy} onClick={() => void accept(item)}><Check size={16} className="mr-1 inline" />Accept</button><button className="btn-secondary px-3 py-2 text-sm" disabled={busy} onClick={() => { setTarget(item); setReason(""); }}><X size={16} className="mr-1 inline" />Reject</button></div></div>)}</div> : <div className="p-10 text-center text-sm text-slate-400">No pending appointment requests.</div>}
    </section>
    {target && <Modal title="Reject appointment request" onClose={() => setTarget(null)}><form onSubmit={reject} className="space-y-4"><p className="text-sm text-slate-500">Give the patient a reason for rejecting this request.</p><label className="block"><span className="label">Reason *</span><textarea className="input min-h-28" value={reason} required maxLength={1000} onChange={(e) => setReason(e.target.value)} placeholder="e.g. Clinic unavailable at this time" /></label><div className="flex justify-end gap-2"><button type="button" className="btn-secondary" onClick={() => setTarget(null)}>Cancel</button><button className="btn-primary" disabled={busy || !reason.trim()}>{busy ? "Rejecting..." : "Reject request"}</button></div></form></Modal>}
  </div>;
}
