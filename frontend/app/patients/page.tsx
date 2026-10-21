"use client";

import { FormEvent, useCallback, useMemo, useState } from "react";
import { Mail, Phone, Plus, Search } from "lucide-react";
import { createPatient, listPatients, listRecords } from "@/lib/api/patient";
import { usePolling } from "@/lib/hooks";
import { Patient, RecordItem } from "@/lib/types";
import { Overlay } from "@/components/Overlay";
import { useToast } from "@/components/Toast";

const initial = { name: "", dateOfBirth: "", gender: "", phone: "", email: "" };
export default function PatientsPage() {
  const load = useCallback(() => listPatients(), []);
  const { data, loading, error } = usePolling(load);
  const patients = useMemo(() => data?.data ?? [], [data]);
  const [query, setQuery] = useState("");
  const [form, setForm] = useState(initial);
  const [formError, setFormError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [selected, setSelected] = useState<Patient | null>(null);
  const filtered = useMemo(() => patients.filter((p) => `${p.name} ${p.email} ${p.phone}`.toLowerCase().includes(query.toLowerCase())), [patients, query]);
  const toast = useToast();
  async function submit(event: FormEvent) {
    event.preventDefault(); setFormError("");
    if (!form.name.trim() || !form.gender.trim() || !form.phone.trim() || !form.dateOfBirth || new Date(form.dateOfBirth) > new Date()) { setFormError("Name, date of birth, gender, and phone are required. Birth date cannot be in the future."); return; }
    if (form.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email)) { setFormError("Enter a valid email address."); return; }
    setSubmitting(true);
    try { await createPatient(form); setForm(initial); toast("Patient registered successfully."); }
    catch (e) { setFormError(e instanceof Error ? e.message : "Unable to register patient."); }
    finally { setSubmitting(false); }
  }
  return <div className="space-y-7"><div className="flex flex-col justify-between gap-4 md:flex-row md:items-end"><div><p className="text-sm text-slate-500">Patient & records service</p><h2 className="mt-1 text-2xl font-bold text-navy">Patient directory</h2></div><div className="relative w-full md:w-80"><Search className="absolute left-3 top-3 text-slate-400" size={17} /><input className="input pl-10" placeholder="Search patients..." value={query} onChange={(e) => setQuery(e.target.value)} /></div></div>
    <div className="grid gap-6 xl:grid-cols-[360px_1fr]"><section className="card p-6"><div className="mb-5 flex items-center gap-3"><div className="grid h-10 w-10 place-items-center rounded-xl bg-teal/10 text-teal"><Plus size={20} /></div><div><h3 className="font-bold text-navy">Register patient</h3><p className="text-xs text-slate-400">Add a new care profile</p></div></div><form onSubmit={submit} className="space-y-4"><Field label="Full name" value={form.name} onChange={(value) => setForm({ ...form, name: value })} required /><div className="grid grid-cols-2 gap-3"><Field label="Date of birth" type="date" value={form.dateOfBirth} onChange={(value) => setForm({ ...form, dateOfBirth: value })} required /><Field label="Gender" value={form.gender} onChange={(value) => setForm({ ...form, gender: value })} placeholder="e.g. Female" required /></div><Field label="Phone" value={form.phone} onChange={(value) => setForm({ ...form, phone: value })} required /><Field label="Email (optional)" type="email" value={form.email} onChange={(value) => setForm({ ...form, email: value })} />{formError && <p className="rounded-lg bg-red/5 px-3 py-2 text-xs text-red">{formError}</p>}<button className="btn-primary w-full" disabled={submitting}>{submitting ? "Registering..." : "Register patient"}</button></form></section><section className="card overflow-hidden"><div className="border-b border-line px-6 py-5"><h3 className="font-bold text-navy">All patients <span className="ml-2 rounded-full bg-canvas px-2 py-1 text-xs text-slate-500">{filtered.length}</span></h3></div>{loading && !data ? <SkeletonRows /> : error ? <Empty text="Patient service is unavailable. Retrying automatically..." /> : filtered.length ? <div className="divide-y divide-line">{filtered.map((patient) => <button key={patient.id} onClick={() => setSelected(patient)} className="flex w-full items-center gap-4 px-6 py-4 text-left transition hover:bg-canvas"><div className="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-teal/10 font-bold text-teal">{patient.name.slice(0, 1).toUpperCase()}</div><div className="min-w-0 flex-1"><p className="truncate text-sm font-bold text-navy">{patient.name}</p><p className="mt-1 text-xs text-slate-500">{patient.gender} · Born {patient.dateOfBirth}</p></div><div className="hidden text-right text-xs text-slate-400 sm:block">{patient.email && <p>{patient.email}</p>}<p>{patient.phone}</p></div></button>)}</div> : <Empty text="No patients match your search." />}</section></div>
    {selected && <PatientDrawer patient={selected} onClose={() => setSelected(null)} />}
  </div>;
}
function Field({ label, value, onChange, type = "text", placeholder, required = false }: { label: string; value: string; onChange: (value: string) => void; type?: string; placeholder?: string; required?: boolean }) { return <label className="block"><span className="label">{label}{required && " *"}</span><input className="input" type={type} value={value} placeholder={placeholder} required={required} onChange={(e) => onChange(e.target.value)} /></label>; }
function Empty({ text }: { text: string }) { return <div className="px-6 py-12 text-center text-sm text-slate-400">{text}</div>; }
function SkeletonRows() { return <div className="space-y-4 p-6">{[1, 2, 3].map((item) => <div key={item} className="flex animate-pulse gap-4"><div className="h-10 w-10 rounded-full bg-canvas" /><div className="flex-1 space-y-2"><div className="h-3 w-1/3 rounded bg-canvas" /><div className="h-3 w-1/2 rounded bg-canvas" /></div></div>)}</div>; }
function PatientDrawer({ patient, onClose }: { patient: Patient; onClose: () => void }) {
  const load = useCallback(() => listRecords(patient.id), [patient.id]);
  const { data, loading } = usePolling(load);
  const records: RecordItem[] = data?.data ?? [];
  return <Overlay title="Patient profile" onClose={onClose} wide><div className="rounded-2xl bg-gradient-to-br from-teal/10 to-blue/10 p-5"><div className="flex items-center gap-4"><div className="grid h-14 w-14 place-items-center rounded-full bg-teal text-xl font-bold text-white">{patient.name.slice(0, 1)}</div><div><h3 className="text-lg font-bold text-navy">{patient.name}</h3><p className="text-sm text-slate-500">{patient.gender} · Born {patient.dateOfBirth}</p></div></div><div className="mt-5 grid gap-2 text-sm text-slate-600"><span className="flex items-center gap-2"><Phone size={15} />{patient.phone}</span><span className="flex items-center gap-2"><Mail size={15} />{patient.email || "No email on file"}</span></div></div><h3 className="mt-8 font-bold text-navy">Medical record history</h3>{loading ? <p className="mt-4 text-sm text-slate-400">Loading records...</p> : records.length ? <div className="mt-4 space-y-4">{records.map((record) => <div key={record.id} className="rounded-xl border border-line p-4"><div className="flex justify-between gap-3"><p className="font-semibold text-navy">Visit with Dr. {record.doctorName}</p><span className="text-xs text-slate-400">{new Date(record.visitDate).toLocaleDateString()}</span></div><p className="mt-3 text-sm text-slate-600"><b>Diagnosis:</b> {record.diagnosis}</p><p className="mt-1 text-sm text-slate-600"><b>Prescription:</b> {record.prescription}</p></div>)}</div> : <div className="mt-4 rounded-xl bg-canvas p-5 text-sm text-slate-500">No medical records have been created yet.</div>}</Overlay>;
}
