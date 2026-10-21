"use client";

import { FormEvent, useCallback, useState } from "react";
import { useRouter } from "next/navigation";
import { HeartPulse, ShieldCheck, Stethoscope, UserRound } from "lucide-react";
import { listDoctors } from "@/lib/api/doctor";
import { createPatient, listPatients } from "@/lib/api/patient";
import { usePolling } from "@/lib/hooks";
import { useAuth, Role } from "@/components/AuthContext";
import { homeFor } from "@/components/RoleGuard";

export default function LoginPage() {
  const { signIn } = useAuth();
  const router = useRouter();
  const patients = usePolling(useCallback(() => listPatients(), []));
  const doctors = usePolling(useCallback(() => listDoctors(), []));
  const [patientId, setPatientId] = useState("");
  const [doctorId, setDoctorId] = useState("");
  const [showRegister, setShowRegister] = useState(false);
  const [form, setForm] = useState({ name: "", email: "", dateOfBirth: "", gender: "", phone: "" });
  const [message, setMessage] = useState("");

  function enter(role: Role, userId: string, name: string) {
    if (!userId || !name) { setMessage("Choose an identity before continuing."); return; }
    signIn({ role, userId, name });
    router.push(homeFor(role));
  }
  async function register(event: FormEvent) {
    event.preventDefault();
    try {
      const patient = await createPatient(form);
      signIn({ role: "PATIENT", userId: patient.id, name: patient.name });
      router.push(homeFor("PATIENT"));
    } catch (error) { setMessage(error instanceof Error ? error.message : "Unable to register patient."); }
  }
  return <main className="min-h-screen bg-canvas px-5 py-10 sm:px-8"><div className="mx-auto max-w-5xl">
    <div className="mb-10 text-center"><div className="mx-auto grid h-14 w-14 place-items-center rounded-2xl bg-teal text-white shadow-card"><HeartPulse size={30} /></div><p className="mt-4 text-sm font-bold uppercase tracking-[0.25em] text-teal">MediCare HMS</p><h1 className="mt-2 text-3xl font-bold text-navy sm:text-4xl">Choose your care workspace</h1><p className="mx-auto mt-3 max-w-xl text-sm text-slate-500">This lab uses simulated identities. Select a role to continue.</p></div>
    {message && <div className="mx-auto mb-5 max-w-xl rounded-lg border border-red/20 bg-red/5 px-4 py-3 text-center text-sm text-red">{message}</div>}
    <div className="grid gap-5 md:grid-cols-3">
      <RoleCard icon={<UserRound />} title="Patient" text="Book care, follow appointments, and view records." color="teal"><select className="input" value={patientId} onChange={(event) => setPatientId(event.target.value)}><option value="">Choose patient</option>{(patients.data?.data ?? []).map((patient) => <option key={patient.id} value={patient.id}>{patient.name}</option>)}</select><button className="btn-primary mt-3 w-full" onClick={() => { const patient = patients.data?.data.find((item) => item.id === patientId); enter("PATIENT", patientId, patient?.name ?? ""); }}>Enter as patient</button><button className="mt-3 w-full text-xs font-bold text-teal hover:underline" onClick={() => setShowRegister(!showRegister)}>Register new patient</button></RoleCard>
      <RoleCard icon={<Stethoscope />} title="Doctor" text="Review incoming requests and manage your schedule." color="blue"><select className="input" value={doctorId} onChange={(event) => setDoctorId(event.target.value)}><option value="">Choose doctor</option>{(doctors.data ?? []).map((doctor) => <option key={doctor.id} value={doctor.id}>{doctor.name}</option>)}</select><button className="btn-primary mt-3 w-full" onClick={() => { const doctor = doctors.data?.find((item) => item.id === doctorId); enter("DOCTOR", doctorId, doctor?.name ?? ""); }}>Enter as doctor</button></RoleCard>
      <RoleCard icon={<ShieldCheck />} title="Admin" text="View the complete operational dashboard and all activity." color="navy"><button className="btn-primary mt-6 w-full" onClick={() => enter("ADMIN", "admin", "Hospital administrator")}>Enter as admin</button></RoleCard>
    </div>
    {showRegister && <form onSubmit={register} className="mx-auto mt-6 max-w-xl card grid gap-4 p-6 sm:grid-cols-2"><h2 className="sm:col-span-2 text-lg font-bold text-navy">Register new patient</h2>{[["name","Full name"],["email","Email"],["dateOfBirth","Date of birth"],["gender","Gender"],["phone","Phone"]].map(([key, label]) => <label key={key}><span className="label">{label} *</span><input className="input" required type={key === "dateOfBirth" ? "date" : key === "email" ? "email" : "text"} value={form[key as keyof typeof form]} onChange={(event) => setForm({ ...form, [key]: event.target.value })} /></label>)}<button className="btn-primary sm:col-span-2">Register and continue</button></form>}
  </div></main>;
}
function RoleCard({ icon, title, text, color, children }: { icon: React.ReactNode; title: string; text: string; color: string; children: React.ReactNode }) {
  const iconStyle = color === "teal" ? "bg-teal/10 text-teal" : color === "blue" ? "bg-blue/10 text-blue" : "bg-navy/10 text-navy";
  return <section className="card p-6"><div className={`grid h-11 w-11 place-items-center rounded-xl ${iconStyle}`}>{icon}</div><h2 className="mt-5 text-xl font-bold text-navy">{title}</h2><p className="mt-2 min-h-10 text-sm leading-5 text-slate-500">{text}</p><div className="mt-5">{children}</div></section>;
}
