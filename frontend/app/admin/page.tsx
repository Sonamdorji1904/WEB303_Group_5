"use client";

import { useCallback } from "react";
import { listPatients } from "@/lib/api/patient";
import { listDoctors } from "@/lib/api/doctor";
import { listAppointments } from "@/lib/api/appointment";
import { usePolling } from "@/lib/hooks";
import { StatusBadge } from "@/components/StatusBadge";

export default function AdminDashboard() {
  const load = useCallback(() => Promise.all([listPatients(), listDoctors(), listAppointments()]), []);
  const { data, loading } = usePolling(load);
  const [patients, doctors, appointments] = data ?? [{ data: [] }, [], []] as const;
  const stats = [["Patients", patients.data.length], ["Doctors", doctors.length], ["Appointments", appointments.length], ["Pending", appointments.filter((item) => item.status === "PENDING").length], ["Confirmed", appointments.filter((item) => item.status === "CONFIRMED").length], ["Completed", appointments.filter((item) => item.status === "COMPLETED").length], ["Rejected", appointments.filter((item) => item.status === "REJECTED").length]];
  return <div className="space-y-7"><section className="rounded-2xl bg-gradient-to-br from-teal to-[#087f86] p-8 text-white shadow-card"><p className="text-sm text-white/75">Admin workspace</p><h2 className="mt-2 text-3xl font-bold">Good morning, care team.</h2><p className="mt-3 text-sm text-white/80">A complete operational view of the hospital system.</p></section><div className="grid grid-cols-2 gap-4 md:grid-cols-4 xl:grid-cols-7">{stats.map(([label, value]) => <div key={label} className="card p-4"><p className="text-2xl font-bold text-navy">{loading ? "—" : value}</p><p className="mt-1 text-xs font-medium text-slate-500">{label}</p></div>)}</div><section className="card p-6"><h3 className="font-bold text-navy">Appointments by status</h3><div className="mt-6 grid gap-4 sm:grid-cols-4">{["PENDING", "CONFIRMED", "COMPLETED", "REJECTED"].map((status) => <div key={status} className="rounded-xl bg-canvas p-4"><StatusBadge status={status} /><p className="mt-3 text-2xl font-bold text-navy">{appointments.filter((item) => item.status === status).length}</p></div>)}</div></section></div>;
}
