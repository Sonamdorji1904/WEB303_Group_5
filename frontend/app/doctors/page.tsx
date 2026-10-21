"use client";

import { FormEvent, useCallback, useMemo, useState } from "react";
import { CalendarPlus, Mail, Phone, Plus, Search, Stethoscope } from "lucide-react";
import { useRouter } from "next/navigation";
import { createDoctor, createSlot, listDoctors, listSlots } from "@/lib/api/doctor";
import { useAuth } from "@/components/AuthContext";
import { usePolling } from "@/lib/hooks";
import { Doctor, Slot } from "@/lib/types";
import { StatusBadge } from "@/components/StatusBadge";
import { useToast } from "@/components/Toast";

const doctorInitial = { name: "", specialization: "", email: "", phone: "" };
export default function DoctorsPage() {
  const { identity } = useAuth();
  if (identity?.role === "PATIENT") return <PatientDoctors />;
  if (identity?.role === "DOCTOR") return <DoctorSchedule />;
  return <AdminDoctors />;
}

function PatientDoctors() {
  const router = useRouter();
  const { data: doctors, loading, error } = usePolling(useCallback(() => listDoctors(), []));
  const [search, setSearch] = useState("");
  const filtered = useMemo(() => (doctors ?? []).filter((doctor) => `${doctor.name} ${doctor.specialization}`.toLowerCase().includes(search.toLowerCase())), [doctors, search]);
  return <div className="space-y-7"><Header eyebrow="Patient workspace" title="Find doctors" /><div className="relative max-w-xl"><Search className="absolute left-3 top-3 text-slate-400" size={18} /><input className="input pl-10" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search by doctor name or specialization" /></div>{error && <ErrorText text="Doctor service is unavailable. Retrying automatically..." />}{loading && !doctors ? <Loading text="Loading doctors..." /> : filtered.length ? <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-3">{filtered.map((doctor) => <DoctorCard key={doctor.id} doctor={doctor} availableOnly onBook={(slot) => router.push(`/patient/book?doctorId=${doctor.id}&slotId=${slot.id}`)} />)}</div> : <Empty text="No doctors match your search." />}</div>;
}

function DoctorSchedule() {
  const { identity } = useAuth();
  const doctorId = identity?.userId ?? "";
  const load = useCallback(() => listSlots(doctorId), [doctorId]);
  const { data: slots, loading, error } = usePolling(load);
  const [form, setForm] = useState({ date: "", startTime: "" });
  const [busy, setBusy] = useState(false);
  const toast = useToast();
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!form.date || !form.startTime) { toast("Choose a date and start time.", "error"); return; }
    if (form.date < new Date().toISOString().slice(0, 10)) { toast("Availability date cannot be in the past.", "error"); return; }
    setBusy(true);
    try { await createSlot(doctorId, form); setForm({ date: "", startTime: "" }); toast("Availability slot added."); }
    catch (e) { toast(e instanceof Error ? e.message : "Unable to add availability.", "error"); }
    finally { setBusy(false); }
  }
  const sorted = [...(slots ?? [])].sort((a, b) => `${a.date}${a.startTime}`.localeCompare(`${b.date}${b.startTime}`));
  return <div className="space-y-7"><Header eyebrow="Doctor workspace" title="My schedule" /><section className="card p-6"><FormHeading icon={<CalendarPlus size={20} />} title="Add your availability" subtitle="Offer a new appointment slot to patients" /><form onSubmit={submit} className="grid gap-4 sm:grid-cols-3"><Field label="Date" type="date" value={form.date} onChange={(v) => setForm({ ...form, date: v })} required /><Field label="Start time" type="time" value={form.startTime} onChange={(v) => setForm({ ...form, startTime: v })} required /><button className="btn-primary self-end" disabled={busy}>{busy ? "Saving..." : "Add slot"}</button></form></section><section className="card p-6"><h3 className="font-bold text-navy">My availability</h3>{error ? <ErrorText text="Doctor service is unavailable. Retrying automatically..." /> : loading && !slots ? <Loading text="Loading your slots..." /> : sorted.length ? <div className="mt-4 space-y-2">{sorted.map((slot) => <div key={slot.id} className="flex items-center justify-between rounded-lg bg-canvas px-4 py-3"><div><p className="text-sm font-semibold text-navy">{slot.date} · {slot.startTime}</p>{slot.status === "HELD" && <p className="mt-1 text-xs text-amber">Awaiting your decision</p>}</div><StatusBadge status={slot.status} /></div>)}</div> : <p className="mt-4 rounded-lg bg-canvas px-4 py-8 text-center text-sm text-slate-400">No slots yet. Add your first availability above.</p>}</section></div>;
}

function AdminDoctors() {
  const { data: doctors, loading, error } = usePolling(useCallback(() => listDoctors(), []));
  const [form, setForm] = useState(doctorInitial);
  const [slotTarget, setSlotTarget] = useState("");
  const [slotForm, setSlotForm] = useState({ date: "", startTime: "" });
  const [busy, setBusy] = useState(false);
  const [errorText, setErrorText] = useState("");
  const toast = useToast();
  async function addDoctor(event: FormEvent) { event.preventDefault(); setErrorText(""); if (!form.name.trim() || !form.specialization.trim() || !form.phone.trim()) { setErrorText("Name, specialization, and phone are required."); return; } setBusy(true); try { await createDoctor(form); setForm(doctorInitial); toast("Doctor added successfully."); } catch (e) { setErrorText(e instanceof Error ? e.message : "Unable to add doctor."); } finally { setBusy(false); } }
  async function addSlot(event: FormEvent) { event.preventDefault(); setErrorText(""); if (!slotTarget || !slotForm.date || !slotForm.startTime) { setErrorText("Choose a doctor, date, and time."); return; } setBusy(true); try { await createSlot(slotTarget, slotForm); setSlotForm({ date: "", startTime: "" }); toast("Availability slot added."); } catch (e) { setErrorText(e instanceof Error ? e.message : "Unable to add slot."); } finally { setBusy(false); } }
  return <div className="space-y-7"><Header eyebrow="Admin workspace" title="Doctors & slots" />{errorText && <ErrorText text={errorText} />}<div className="grid gap-6 xl:grid-cols-2"><section className="card p-6"><FormHeading icon={<Plus size={20} />} title="Add doctor" subtitle="Create a provider profile" /><form onSubmit={addDoctor} className="grid gap-4 sm:grid-cols-2"><Field label="Full name" value={form.name} onChange={(v) => setForm({ ...form, name: v })} required /><Field label="Specialization" value={form.specialization} onChange={(v) => setForm({ ...form, specialization: v })} required /><Field label="Email" type="email" value={form.email} onChange={(v) => setForm({ ...form, email: v })} /><Field label="Phone" value={form.phone} onChange={(v) => setForm({ ...form, phone: v })} required /><button className="btn-primary sm:col-span-2" disabled={busy}>{busy ? "Saving..." : "Add doctor"}</button></form></section><section className="card p-6"><FormHeading icon={<CalendarPlus size={20} />} title="Add availability" subtitle="Make a new appointment slot" /><form onSubmit={addSlot} className="grid gap-4 sm:grid-cols-3"><label className="sm:col-span-3"><span className="label">Doctor *</span><select className="input" value={slotTarget} onChange={(e) => setSlotTarget(e.target.value)}><option value="">Choose doctor</option>{(doctors ?? []).map((doctor) => <option key={doctor.id} value={doctor.id}>{doctor.name} · {doctor.specialization}</option>)}</select></label><Field label="Date" type="date" value={slotForm.date} onChange={(v) => setSlotForm({ ...slotForm, date: v })} required /><Field label="Start time" type="time" value={slotForm.startTime} onChange={(v) => setSlotForm({ ...slotForm, startTime: v })} required /><button className="btn-primary self-end" disabled={busy}>{busy ? "Saving..." : "Add slot"}</button></form></section></div><section><div className="mb-4 flex items-center justify-between"><h3 className="text-lg font-bold text-navy">Doctors <span className="ml-2 rounded-full bg-white px-2 py-1 text-xs text-slate-500">{doctors?.length ?? 0}</span></h3>{error && <span className="text-xs text-red">Doctor service unavailable. Retrying...</span>}</div>{loading && !doctors ? <Loading text="Loading care team..." /> : doctors?.length ? <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-3">{doctors.map((doctor) => <DoctorCard key={doctor.id} doctor={doctor} />)}</div> : <Empty text="No doctors have been added yet." />}</section></div>;
}

function DoctorCard({ doctor, availableOnly = false, onBook }: { doctor: Doctor; availableOnly?: boolean; onBook?: (slot: Slot) => void }) {
  const { data: slots, loading } = usePolling(useCallback(() => listSlots(doctor.id, availableOnly ? "AVAILABLE" : undefined), [doctor.id, availableOnly]));
  return <div className="card p-5"><div className="flex items-start gap-3"><div className="grid h-11 w-11 place-items-center rounded-xl bg-blue/10 text-blue"><Stethoscope size={20} /></div><div className="min-w-0"><h4 className="font-bold text-navy">{doctor.name}</h4><p className="text-sm text-teal">{doctor.specialization}</p></div></div><div className="mt-4 space-y-2 text-xs text-slate-500"><p className="flex items-center gap-2"><Mail size={14} />{doctor.email || "No email"}</p><p className="flex items-center gap-2"><Phone size={14} />{doctor.phone}</p></div><div className="mt-5 border-t border-line pt-4"><p className="mb-3 text-xs font-bold uppercase tracking-wide text-slate-500">{availableOnly ? "Available slots" : "Availability"}</p>{loading && !slots ? <Loading text="Loading slots..." /> : slots?.length ? <div className="space-y-2">{slots.map((slot) => <div key={slot.id} className="flex items-center justify-between rounded-lg bg-canvas px-3 py-2"><span className="text-xs font-medium text-navy">{slot.date} · {slot.startTime}</span>{onBook ? <button className="btn-primary px-2 py-1 text-xs" onClick={() => onBook(slot)}>Book this slot</button> : <StatusBadge status={slot.status} />}</div>)}</div> : <p className="text-xs text-slate-400">No available slots.</p>}</div></div>;
}
function Header({ eyebrow, title }: { eyebrow: string; title: string }) { return <div><p className="text-sm text-slate-500">{eyebrow}</p><h2 className="mt-1 text-2xl font-bold text-navy">{title}</h2></div>; }
function FormHeading({ icon, title, subtitle }: { icon: React.ReactNode; title: string; subtitle: string }) { return <div className="mb-5 flex items-center gap-3"><div className="grid h-10 w-10 place-items-center rounded-xl bg-teal/10 text-teal">{icon}</div><div><h3 className="font-bold text-navy">{title}</h3><p className="text-xs text-slate-400">{subtitle}</p></div></div>; }
function Field({ label, value, onChange, type = "text", required = false }: { label: string; value: string; onChange: (value: string) => void; type?: string; required?: boolean }) { return <label className="block"><span className="label">{label}{required && " *"}</span><input className="input" type={type} value={value} required={required} onChange={(e) => onChange(e.target.value)} /></label>; }
function ErrorText({ text }: { text: string }) { return <div className="rounded-lg border border-red/20 bg-red/5 px-4 py-3 text-sm text-red">{text}</div>; }
function Loading({ text }: { text: string }) { return <div className="p-8 text-center text-sm text-slate-400">{text}</div>; }
function Empty({ text }: { text: string }) { return <div className="card p-10 text-center text-sm text-slate-400">{text}</div>; }
