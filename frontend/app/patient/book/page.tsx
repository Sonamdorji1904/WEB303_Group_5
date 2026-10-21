"use client";

import { FormEvent, Suspense, useCallback, useEffect, useState } from "react";
import { ArrowRight, CalendarPlus } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { createAppointment } from "@/lib/api/appointment";
import { listDoctors, listSlots } from "@/lib/api/doctor";
import { getPatient } from "@/lib/api/patient";
import { useAuth } from "@/components/AuthContext";
import { usePolling } from "@/lib/hooks";
import { Slot } from "@/lib/types";
import { useToast } from "@/components/Toast";

function BookForm() {
  const { identity, updateIdentity } = useAuth();
  const router = useRouter();
  const params = useSearchParams();
  const doctorParam = params.get("doctorId") ?? "";
  const slotParam = params.get("slotId") ?? "";
  const { data: doctors } = usePolling(useCallback(() => listDoctors(), []));
  const [doctorId, setDoctorId] = useState(() => doctorParam);
  const [slotId, setSlotId] = useState(slotParam);
  const [reason, setReason] = useState("");
  const [email, setEmail] = useState(identity?.email ?? "");
  const [busy, setBusy] = useState(false);
  const toast = useToast();

  useEffect(() => {
    if (identity?.email) return;
    if (!identity?.userId) return;
    void getPatient(identity.userId).then((patient) => { setEmail(patient.email ?? ""); updateIdentity({ email: patient.email ?? "" }); }).catch(() => undefined);
  }, [identity, updateIdentity]);
  const slotsLoad = useCallback(() => doctorId ? listSlots(doctorId, "AVAILABLE") : Promise.resolve([] as Slot[]), [doctorId]);
  const { data: slots } = usePolling(slotsLoad);
  const doctor = doctors?.find((item) => item.id === doctorId);
  async function submit(event: FormEvent) {
    event.preventDefault();
    const slot = (slots ?? []).find((item) => item.id === slotId);
    if (!identity || !doctor || !slot || !reason.trim()) { toast("Choose an available slot and enter a reason.", "error"); return; }
    setBusy(true);
    try {
      await createAppointment({ patientId: identity.userId, patientName: identity.name, patientEmail: email, doctorId: doctor.id, doctorName: doctor.name, date: slot.date, time: slot.startTime, reason: reason.trim() });
      toast(`Request sent, waiting for Dr. ${doctor.name} to respond`);
      router.push("/patient/appointments");
    } catch (error) { toast(error instanceof Error ? error.message : "Unable to send appointment request.", "error"); }
    finally { setBusy(false); }
  }
  return <div className="space-y-7"><div><p className="text-sm text-slate-500">Patient workspace</p><h2 className="mt-1 text-2xl font-bold text-navy">Book appointment</h2></div><section className="card p-6"><div className="mb-5 flex items-center gap-3"><div className="grid h-10 w-10 place-items-center rounded-xl bg-teal/10 text-teal"><CalendarPlus size={20} /></div><div><h3 className="font-bold text-navy">Booking as {identity?.name}</h3><p className="text-xs text-slate-400">Your request will wait for doctor approval.</p></div></div><form onSubmit={submit} className="grid gap-4 sm:grid-cols-2"><label><span className="label">Doctor *</span><select className="input" value={doctorId} onChange={(e) => { setDoctorId(e.target.value); setSlotId(""); }}><option value="">Choose doctor</option>{(doctors ?? []).map((item) => <option key={item.id} value={item.id}>{item.name} · {item.specialization}</option>)}</select></label><label><span className="label">Available slot *</span><select className="input" value={slotId} onChange={(e) => setSlotId(e.target.value)} disabled={!doctorId}>  <option value="">{doctorId ? "Choose slot" : "Choose doctor first"}</option>{(slots ?? []).map((slot) => <option key={slot.id} value={slot.id}>{slot.date} · {slot.startTime}</option>)}</select></label><label className="sm:col-span-2"><span className="label">Reason *</span><input className="input" required value={reason} onChange={(e) => setReason(e.target.value)} placeholder="Reason for visit" /></label><div className="sm:col-span-2"><button className="btn-primary" disabled={busy}>{busy ? "Sending request..." : "Send appointment request"} <ArrowRight className="ml-2 inline" size={16} /></button></div></form></section></div>;
}

export default function BookPage() {
  return <Suspense fallback={<div className="p-10 text-center text-sm text-slate-400">Loading booking form...</div>}><BookForm /></Suspense>;
}
