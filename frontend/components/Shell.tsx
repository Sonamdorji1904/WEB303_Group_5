"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { Bell, CalendarDays, ChevronLeft, ChevronRight, ClipboardList, HeartPulse, LayoutDashboard, Menu, Stethoscope, Users, X } from "lucide-react";
import { useState } from "react";
import { health as patientHealth } from "@/lib/api/patient";
import { health as doctorHealth } from "@/lib/api/doctor";
import { health as appointmentHealth } from "@/lib/api/appointment";
import { health as notificationHealth } from "@/lib/api/notification";
import { usePolling } from "@/lib/hooks";
import { ToastProvider } from "./Toast";
import { useAuth } from "./AuthContext";

const roleLinks = {
  PATIENT: [
    ["/patient/doctors", "Find doctors", Stethoscope], ["/patient/book", "Book appointment", CalendarDays],
    ["/patient/appointments", "My appointments", ClipboardList], ["/patient/records", "My records", Users], ["/patient/notifications", "Notifications", Bell],
  ],
  DOCTOR: [
    ["/doctor/requests", "Requests", ClipboardList], ["/doctor/schedule", "My schedule", CalendarDays],
    ["/doctor/appointments", "Confirmed appointments", ClipboardList], ["/doctor/notifications", "Notifications", Bell],
  ],
  ADMIN: [
    ["/admin", "Dashboard", LayoutDashboard], ["/admin/doctors", "Doctors", Stethoscope], ["/admin/patients", "Patients", Users],
    ["/admin/appointments", "All appointments", CalendarDays], ["/admin/notifications", "All notifications", Bell],
  ],
} as const;
function ServiceStrip() {
  const patient = usePolling(patientHealth, 5000);
  const doctor = usePolling(doctorHealth, 5000);
  const appointment = usePolling(appointmentHealth, 5000);
  const notification = usePolling(notificationHealth, 5000);
  const checks = [
    ["Patient", patient],
    ["Doctor", doctor],
    ["Appointment", appointment],
    ["Notification", notification],
  ] as const;
  return <div className="hidden items-center gap-4 lg:flex">{checks.map(([label, check]) => <div key={label} className="flex items-center gap-1.5 text-xs font-medium text-slate-500"><span className={`h-2 w-2 rounded-full ${check.data ? "bg-green" : check.loading ? "bg-amber" : "bg-red"}`} />{label}</div>)}</div>;
}

export function Shell({ children }: { children: React.ReactNode }) {
  const { identity, signOut } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  if (!identity || pathname === "/") return <ToastProvider>{children}</ToastProvider>;
  const links = roleLinks[identity.role].map(([href, label, icon]) => ({ href, label, icon }));
  const sidebar = <aside className={`fixed inset-y-0 left-0 z-30 flex flex-col border-r border-line bg-white transition-all ${collapsed ? "w-20" : "w-64"} ${mobileOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0"}`}>
    <div className="flex h-20 items-center gap-3 border-b border-line px-5"><div className="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-teal text-white"><HeartPulse size={23} /></div>{!collapsed && <div><div className="font-bold text-navy">MediCare HMS</div><div className="text-[11px] text-slate-400">Care, connected.</div></div>}<button className="ml-auto rounded-lg p-1 text-slate-400 lg:hidden" onClick={() => setMobileOpen(false)} aria-label="Close navigation"><X size={18} /></button></div>
    <nav className="flex-1 space-y-1 p-4">{links.map(({ href, label, icon: Icon }) => <Link key={href} href={href} onClick={() => setMobileOpen(false)} className={`flex items-center gap-3 rounded-lg px-3 py-3 text-sm font-semibold transition ${pathname === href ? "bg-teal/10 text-teal" : "text-slate-500 hover:bg-canvas hover:text-navy"}`}><Icon size={19} />{!collapsed && label}</Link>)}</nav>
    <button onClick={() => setCollapsed(!collapsed)} className="m-4 hidden items-center justify-center rounded-lg border border-line p-2 text-slate-500 hover:text-teal lg:flex" aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}>{collapsed ? <ChevronRight size={18} /> : <ChevronLeft size={18} />}</button>
  </aside>;
  const title = links.find((link) => link.href === pathname)?.label ?? "Dashboard";
  return <ToastProvider><div className="min-h-screen bg-canvas">{sidebar}<div className={`transition-all ${collapsed ? "lg:pl-20" : "lg:pl-64"}`}><header className="sticky top-0 z-20 flex h-20 items-center justify-between border-b border-line bg-white/90 px-5 backdrop-blur sm:px-8"><div className="flex items-center gap-3"><button className="rounded-lg p-2 text-slate-500 lg:hidden" onClick={() => setMobileOpen(true)} aria-label="Open navigation"><Menu size={22} /></button><div><p className="text-xs font-semibold uppercase tracking-widest text-teal">Hospital operations</p><h1 className="text-lg font-bold text-navy">{title}</h1></div></div><div className="flex items-center gap-3"><div className="hidden text-right sm:block"><p className="text-sm font-bold text-navy">{identity.name}</p><p className="text-[10px] font-bold tracking-widest text-teal">{identity.role}</p></div><button onClick={() => { signOut(); router.push("/"); }} className="rounded-lg border border-line px-3 py-2 text-xs font-bold text-slate-500 hover:text-red">Switch role</button><ServiceStrip /></div></header><main className="mx-auto max-w-[1500px] p-5 sm:p-8">{children}</main></div></div></ToastProvider>;
}
