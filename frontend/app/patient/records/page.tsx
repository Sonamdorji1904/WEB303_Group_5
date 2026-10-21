"use client";

import { useCallback } from "react";
import { listRecords } from "@/lib/api/patient";
import { usePolling } from "@/lib/hooks";
import { useAuth } from "@/components/AuthContext";

export default function RecordsPage() {
  const { identity } = useAuth();
  const load = useCallback(() => listRecords(identity?.userId ?? ""), [identity?.userId]);
  const { data, loading, error } = usePolling(load);
  return <div className="space-y-7"><div><p className="text-sm text-slate-500">Patient workspace</p><h2 className="mt-1 text-2xl font-bold text-navy">My medical records</h2></div><section className="card divide-y divide-line">{loading && !data ? <p className="p-8 text-center text-sm text-slate-400">Loading records...</p> : error ? <p className="p-8 text-center text-sm text-red">Records are unavailable right now.</p> : data?.data.length ? data.data.map((record) => <article key={record.id} className="p-6"><div className="flex items-center justify-between"><h3 className="font-bold text-navy">{record.doctorName}</h3><span className="text-xs text-slate-400">{record.visitDate}</span></div><p className="mt-3 text-sm text-slate-600"><b>Diagnosis:</b> {record.diagnosis}</p><p className="mt-1 text-sm text-slate-600"><b>Prescription:</b> {record.prescription}</p></article>) : <p className="p-8 text-center text-sm text-slate-400">No medical records yet.</p>}</section></div>;
}
