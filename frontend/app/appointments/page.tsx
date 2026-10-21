"use client";

import { FormEvent, useCallback, useMemo, useState } from "react";
import { ArrowRight, CalendarPlus, CheckCircle2, Circle, Filter } from "lucide-react";
import { createAppointment, completeAppointment, listAppointments } from "@/lib/api/appointment";
import { listPatients } from "@/lib/api/patient";
import { listDoctors, listSlots } from "@/lib/api/doctor";
import { listNotifications } from "@/lib/api/notification";
import { usePolling } from "@/lib/hooks";
import { Appointment, Notification, Slot } from "@/lib/types";
import { Modal, Overlay } from "@/components/Overlay";
import { StatusBadge } from "@/components/StatusBadge";
import { useToast } from "@/components/Toast";
import { useAuth } from "@/components/AuthContext";

const emptyBooking = { patientId: "", doctorId: "", slotId: "", reason: "" };
export default function AppointmentsPage() {
  const { identity } = useAuth();
  if (identity?.role === "PATIENT") return <PatientAppointments />;
  if (identity?.role === "DOCTOR") return <DoctorAppointments />;
  return <AdminAppointmentsPage />;
}

function AdminAppointmentsPage() {
  const patientLoad = useCallback(() => listPatients(), []);
  const doctorLoad = useCallback(() => listDoctors(), []);
  const appointmentLoad = useCallback(() => listAppointments(), []);
  const notificationLoad = useCallback(() => listNotifications(), []);
  const { data: patientData } = usePolling(patientLoad);
  const { data: doctors } = usePolling(doctorLoad);
  const { data: appointments, loading, error } = usePolling(appointmentLoad);
  const { data: notifications } = usePolling(notificationLoad);
  const patients = patientData?.data ?? [];
  const [booking, setBooking] = useState(emptyBooking);
  const [reason, setReason] = useState("");
  const [slots, setSlots] = useState<Slot[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [patientFilter, setPatientFilter] = useState("");
  const [selected, setSelected] = useState<Appointment | null>(null);
  const [completeTarget, setCompleteTarget] = useState<Appointment | null>(null);
  const toast = useToast();
  const filtered = (appointments ?? []).filter((item) => (!statusFilter || item.status === statusFilter) && (!patientFilter || item.patientId === patientFilter));
  const selectedPatient = patients.find((item) => item.id === booking.patientId);
  const selectedDoctor = doctors?.find((item) => item.id === booking.doctorId);
  async function chooseDoctor(id: string) { setBooking({ ...booking, doctorId: id, slotId: "" }); setSlots(id ? await listSlots(id, "AVAILABLE") : []); }
  async function submit(event: FormEvent) {
    event.preventDefault(); setMessage("");
    const slot = slots.find((item) => item.id === booking.slotId);
    if (!selectedPatient || !selectedDoctor || !slot || !reason.trim()) { setMessage("Choose a patient, doctor, available slot, and reason."); return; }
    setSubmitting(true);
    try { await createAppointment({ patientId: selectedPatient.id, patientName: selectedPatient.name, patientEmail: selectedPatient.email ?? "", doctorId: selectedDoctor.id, doctorName: selectedDoctor.name, date: slot.date, time: slot.startTime, reason: reason.trim() }); setBooking(emptyBooking); setReason(""); setSlots([]); toast("Request sent, waiting for the doctor service."); }
    catch (e) { setMessage(e instanceof Error ? e.message : "Appointment service is unavailable."); }
    finally { setSubmitting(false); }
  }
  return <div className="space-y-7"><div><p className="text-sm text-slate-500">Appointment service</p><h2 className="mt-1 text-2xl font-bold text-navy">Appointment command center</h2></div><section className="card p-6"><div className="mb-5 flex items-center gap-3"><div className="grid h-10 w-10 place-items-center rounded-xl bg-teal/10 text-teal"><CalendarPlus size={20} /></div><div><h3 className="font-bold text-navy">Book an appointment</h3><p className="text-xs text-slate-400">Availability is reserved through the doctor service</p></div></div><form onSubmit={submit} className="grid gap-4 lg:grid-cols-4"><label><span className="label">Patient *</span><select className="input" value={booking.patientId} onChange={(e) => setBooking({ ...booking, patientId: e.target.value })}><option value="">Choose patient</option>{patients.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label><label><span className="label">Doctor *</span><select className="input" value={booking.doctorId} onChange={(e) => void chooseDoctor(e.target.value)}><option value="">Choose doctor</option>{(doctors ?? []).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label><label><span className="label">Available slot *</span><select className="input" value={booking.slotId} onChange={(e) => setBooking({ ...booking, slotId: e.target.value })} disabled={!booking.doctorId}><option value="">{booking.doctorId ? "Choose slot" : "Choose doctor first"}</option>{slots.map((slot) => <option key={slot.id} value={slot.id}>{slot.date} · {slot.startTime}</option>)}</select></label><label><span className="label">Reason *</span><input className="input" value={reason} onChange={(e) => setReason(e.target.value)} placeholder="Reason for visit" /></label><div className="lg:col-span-4 flex flex-col gap-3 sm:flex-row sm:items-center"><button className="btn-primary" disabled={submitting}>{submitting ? "Sending request..." : "Send appointment request"} <ArrowRight className="ml-2 inline" size={16} /></button>{message && <p className="text-xs text-red">{message}</p>}</div></form></section>
    <section className="card overflow-hidden"><div className="flex flex-col justify-between gap-4 border-b border-line px-6 py-5 md:flex-row md:items-center"><div><h3 className="font-bold text-navy">All appointments</h3><p className="mt-1 text-xs text-slate-400">Click a row to view the event timeline</p></div><div className="flex flex-wrap gap-2"><select className="input w-auto py-2 text-xs" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}><option value="">All statuses</option>{["PENDING", "CONFIRMED", "COMPLETED", "REJECTED"].map((status) => <option key={status}>{status}</option>)}</select><select className="input w-auto py-2 text-xs" value={patientFilter} onChange={(e) => setPatientFilter(e.target.value)}><option value="">All patients</option>{patients.map((patient) => <option key={patient.id} value={patient.id}>{patient.name}</option>)}</select><Filter size={18} className="mt-2 text-slate-400" /></div></div>{loading && !appointments ? <div className="p-8 text-center text-sm text-slate-400">Loading appointments...</div> : error ? <div className="p-8 text-center text-sm text-red">Appointment service is unavailable. Retrying automatically...</div> : filtered.length ? <div className="overflow-x-auto"><table className="w-full min-w-[760px] text-left"><thead className="bg-canvas text-xs uppercase tracking-wide text-slate-500"><tr><th className="px-6 py-3">Patient</th><th className="px-6 py-3">Doctor</th><th className="px-6 py-3">Date & time</th><th className="px-6 py-3">Reason</th><th className="px-6 py-3">Status</th><th className="px-6 py-3 text-right">Action</th></tr></thead><tbody className="divide-y divide-line">{filtered.map((item) => <tr key={item.id} className="cursor-pointer transition hover:bg-canvas" onClick={() => setSelected(item)}><td className="px-6 py-4 text-sm font-semibold text-navy">{item.patientName}</td><td className="px-6 py-4 text-sm text-slate-500">Dr. {item.doctorName}</td><td className="px-6 py-4 text-sm text-slate-500">{item.date}<br /><span className="text-xs">{item.time}</span></td>    <td className="max-w-[220px] px-6 py-4 text-sm text-slate-500"><div className="truncate">{item.reason}</div>{item.status === "REJECTED" && item.rejectionReason && <div className="mt-1 text-xs text-red">Rejected: {item.rejectionReason}</div>}</td><td className="px-6 py-4"><StatusBadge status={item.status} pulse={item.status === "PENDING"} /></td><td className="px-6 py-4 text-right">{item.status === "CONFIRMED" && <button className="btn-secondary px-3 py-1.5 text-xs" onClick={(e) => { e.stopPropagation(); setCompleteTarget(item); }}>Complete visit</button>}</td></tr>)}</tbody></table></div> : <div className="p-10 text-center text-sm text-slate-400">No appointments match these filters.</div>}</section>
    {selected && <AppointmentDrawer appointment={selected} notifications={notifications ?? []} onClose={() => setSelected(null)} onComplete={() => { setSelected(null); setCompleteTarget(selected); }} />}
    {completeTarget && <CompleteModal appointment={completeTarget} onClose={() => setCompleteTarget(null)} />}
  </div>;
}

function PatientAppointments() {
  const { identity } = useAuth();
  const [status, setStatus] = useState("");
  const load = useCallback(() => listAppointments("", identity?.userId ?? ""), [identity?.userId]);
  const { data, loading, error } = usePolling(load);
  const items = (data ?? []).filter((item) => item.patientId === identity?.userId && (!status || item.status === status));
  return <div className="space-y-7"><div><p className="text-sm text-slate-500">Patient workspace</p><h2 className="mt-1 text-2xl font-bold text-navy">My appointments</h2></div><section className="card overflow-hidden"><div className="flex items-center justify-between border-b border-line px-6 py-5"><h3 className="font-bold text-navy">Appointments</h3><select className="input w-auto py-2 text-xs" value={status} onChange={(e) => setStatus(e.target.value)}><option value="">All statuses</option>{["PENDING", "CONFIRMED", "COMPLETED", "REJECTED"].map((item) => <option key={item}>{item}</option>)}</select></div>{loading && !data ? <Loading text="Loading appointments..." /> : error ? <ErrorText text="Appointment service is unavailable. Retrying automatically..." /> : items.length ? <div className="divide-y divide-line">{items.map((item) => <div key={item.id} className="flex flex-col gap-3 px-6 py-5 sm:flex-row sm:items-center sm:justify-between"><div><p className="font-semibold text-navy">Dr. {item.doctorName}</p><p className="mt-1 text-sm text-slate-500">{item.date} · {item.time} · {item.reason}</p>{item.status === "REJECTED" && item.rejectionReason && <p className="mt-1 text-xs text-red">Rejected: {item.rejectionReason}</p>}</div><StatusBadge status={item.status} pulse={item.status === "PENDING"} /></div>)}</div> : <Empty text="No appointments yet." />}</section></div>;
}

function DoctorAppointments() {
  const { identity } = useAuth();
  const load = useCallback(() => listAppointments("", "", identity?.userId ?? ""), [identity?.userId]);
  const { data, loading, error } = usePolling(load);
  const [target, setTarget] = useState<Appointment | null>(null);
  const own = (data ?? []).filter((item) => item.doctorId === identity?.userId);
  const upcoming = own.filter((item) => item.status === "CONFIRMED");
  const history = own.filter((item) => item.status === "COMPLETED");
  return <div className="space-y-7"><div><p className="text-sm text-slate-500">Doctor workspace</p><h2 className="mt-1 text-2xl font-bold text-navy">Confirmed appointments</h2></div>{error && <ErrorText text="Appointment service is unavailable. Retrying automatically..." />}{loading && !data ? <Loading text="Loading appointments..." /> : <><DoctorAppointmentSection title="Upcoming (confirmed)" items={upcoming} onComplete={setTarget} /><DoctorAppointmentSection title="History" items={history} /></>}{target && <CompleteModal appointment={target} onClose={() => setTarget(null)} />}</div>;
}

function DoctorAppointmentSection({ title, items, onComplete }: { title: string; items: Appointment[]; onComplete?: (item: Appointment) => void }) {
  return <section className="card overflow-hidden"><div className="border-b border-line px-6 py-5"><h3 className="font-bold text-navy">{title}</h3></div>{items.length ? <div className="divide-y divide-line">{items.map((item) => <div key={item.id} className="flex flex-col gap-3 px-6 py-5 sm:flex-row sm:items-center sm:justify-between"><div><p className="font-semibold text-navy">{item.patientName}</p><p className="mt-1 text-sm text-slate-500">{item.date} · {item.time} · {item.reason}</p>{item.status === "COMPLETED" && <p className="mt-2 text-xs text-slate-500">Diagnosis: {item.diagnosis} · Prescription: {item.prescription}</p>}</div>{onComplete && <button className="btn-secondary px-3 py-2 text-xs" onClick={() => onComplete(item)}>Complete visit</button>}</div>)}</div> : <p className="p-10 text-center text-sm text-slate-400">{title === "History" ? "No completed visits yet." : "No confirmed appointments yet."}</p>}</section>;
}

function Loading({ text }: { text: string }) { return <div className="p-8 text-center text-sm text-slate-400">{text}</div>; }
function Empty({ text }: { text: string }) { return <div className="p-10 text-center text-sm text-slate-400">{text}</div>; }
function ErrorText({ text }: { text: string }) { return <div className="rounded-lg border border-red/20 bg-red/5 px-4 py-3 text-sm text-red">{text}</div>; }

function AppointmentDrawer({ appointment, notifications, onClose, onComplete }: { appointment: Appointment; notifications: Notification[]; onClose: () => void; onComplete: () => void }) {
  const events = useMemo(() => { const relevant = notifications.filter((item) => item.recipientId === appointment.patientId && item.message.toLowerCase().includes(appointment.doctorName.toLowerCase())); const types = new Set(relevant.map((item) => item.eventType)); return ["AppointmentRequested", appointment.status === "REJECTED" ? "SlotRejected" : "SlotReserved", appointment.status === "REJECTED" ? "AppointmentRejected" : "AppointmentConfirmed", "AppointmentCompleted", "MedicalRecordCreated"].map((type) => ({ type, done: types.has(type) || (type === "AppointmentRequested" && ["PENDING", "CONFIRMED", "COMPLETED", "REJECTED"].includes(appointment.status)) || (type === "SlotReserved" && appointment.status !== "REJECTED") || (type === "AppointmentConfirmed" && ["CONFIRMED", "COMPLETED"].includes(appointment.status)) || (type === "AppointmentRejected" && appointment.status === "REJECTED") || (type === "AppointmentCompleted" && appointment.status === "COMPLETED") })); }, [appointment, notifications]);
  return <Overlay title="Appointment details" onClose={onClose} wide><div className="rounded-2xl bg-gradient-to-br from-teal/10 to-blue/10 p-5"><div className="flex items-start justify-between gap-4"><div><p className="text-xs font-semibold uppercase tracking-wide text-teal">Appointment</p><h3 className="mt-1 text-xl font-bold text-navy">{appointment.patientName}</h3><p className="mt-1 text-sm text-slate-500">with Dr. {appointment.doctorName}</p></div><StatusBadge status={appointment.status} pulse={appointment.status === "PENDING"} /></div><div className="mt-5 grid grid-cols-2 gap-4 text-sm"><span><b className="block text-xs text-slate-400">Date</b>{appointment.date}</span><span><b className="block text-xs text-slate-400">Time</b>{appointment.time}</span><span className="col-span-2"><b className="block text-xs text-slate-400">Reason</b>{appointment.reason}</span></div></div>{appointment.status === "CONFIRMED" && <button className="btn-primary mt-5 w-full" onClick={onComplete}>Complete visit</button>}{appointment.status === "REJECTED" && <p className="mt-5 rounded-lg bg-red/5 p-3 text-sm text-red">{appointment.rejectionReason || "The requested slot was unavailable."}</p>}<h3 className="mt-8 font-bold text-navy">Event timeline</h3><div className="mt-5 space-y-0">{events.map((event, index) => <div key={event.type} className="flex gap-4"><div className="flex flex-col items-center"><div className={`grid h-8 w-8 place-items-center rounded-full ${event.done ? "bg-teal text-white" : "bg-canvas text-slate-400"}`}>{event.done ? <CheckCircle2 size={17} /> : <Circle size={17} />}</div>{index < events.length - 1 && <div className={`h-10 w-px ${event.done ? "bg-teal/40" : "bg-line"}`} />}</div><div className="pt-1"><p className={`text-sm font-semibold ${event.done ? "text-navy" : "text-slate-400"}`}>{event.type}</p><p className="mt-1 text-xs text-slate-400">{event.done ? "Event processed" : "Waiting for this event"}</p></div></div>)}</div></Overlay>;
}

function CompleteModal({ appointment, onClose }: { appointment: Appointment; onClose: () => void }) {
  const [form, setForm] = useState({ diagnosis: "", prescription: "", notes: "" });
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const toast = useToast();
  async function submit(event: FormEvent) { event.preventDefault(); if (!form.diagnosis.trim() || !form.prescription.trim()) { setError("Diagnosis and prescription are required. Use “None” when no medication is needed."); return; } setBusy(true); try { await completeAppointment(appointment.id, form); toast("Visit completed. Medical record creation is now in progress."); onClose(); } catch (e) { setError(e instanceof Error ? e.message : "Unable to complete the visit."); } finally { setBusy(false); } }
  return <Modal title="Complete visit" onClose={onClose}><p className="mb-5 text-sm text-slate-500">Record clinical details for {appointment.patientName}.</p><form onSubmit={submit} className="space-y-4"><TextArea label="Diagnosis *" value={form.diagnosis} onChange={(v) => setForm({ ...form, diagnosis: v })} /><TextArea label="Prescription *" value={form.prescription} onChange={(v) => setForm({ ...form, prescription: v })} placeholder="Use None if no medication is needed" /><TextArea label="Notes" value={form.notes} onChange={(v) => setForm({ ...form, notes: v })} />{error && <p className="rounded-lg bg-red/5 p-3 text-xs text-red">{error}</p>}<button className="btn-primary w-full" disabled={busy}>{busy ? "Saving visit..." : "Complete visit"}</button></form></Modal>;
}
function TextArea({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (value: string) => void; placeholder?: string }) { return <label className="block"><span className="label">{label}</span><textarea className="input min-h-24 resize-y" value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} /></label>; }
