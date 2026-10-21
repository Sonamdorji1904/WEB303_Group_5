"use client";

import { useAuth, Role } from "./AuthContext";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

export function RoleGuard({ role, children }: { role: Role; children: React.ReactNode }) {
  const { identity, ready } = useAuth();
  const router = useRouter();
  useEffect(() => {
    if (!ready) return;
    if (!identity) router.replace("/");
    else if (identity.role !== role) router.replace(homeFor(identity.role));
  }, [identity, ready, role, router]);
  if (!ready || !identity || identity.role !== role) return <div className="grid min-h-[60vh] place-items-center text-sm text-slate-400">Checking access...</div>;
  return <>{children}</>;
}

export function homeFor(role: Role) {
  return role === "PATIENT" ? "/patient/doctors" : role === "DOCTOR" ? "/doctor/requests" : "/admin";
}

export function Forbidden() {
  return <div className="mx-auto max-w-lg rounded-2xl border border-line bg-white p-10 text-center shadow-card"><h2 className="text-2xl font-bold text-navy">403 / Not allowed</h2><p className="mt-3 text-sm text-slate-500">This area is not available for your simulated role.</p></div>;
}
