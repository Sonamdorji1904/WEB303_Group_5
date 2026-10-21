"use client";

import { Role } from "@/components/AuthContext";
import { RoleGuard } from "@/components/RoleGuard";

export function RoleLayout({ role, children }: { role: Role; children: React.ReactNode }) {
  return <RoleGuard role={role}>{children}</RoleGuard>;
}
