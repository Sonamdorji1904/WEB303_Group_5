"use client";

import { createContext, useContext, useMemo, useSyncExternalStore } from "react";

export type Role = "PATIENT" | "DOCTOR" | "ADMIN";
export type Identity = { role: Role; userId: string; name: string; email?: string };

type AuthValue = {
  identity: Identity | null;
  ready: boolean;
  signIn: (identity: Identity) => void;
  updateIdentity: (patch: Partial<Identity>) => void;
  signOut: () => void;
};

const AuthContext = createContext<AuthValue | null>(null);
const storageKey = "medicare.identity";
const listeners = new Set<() => void>();
let cachedRaw: string | null | undefined;
let cachedIdentity: Identity | null = null;

function readIdentity(): Identity | null {
  if (typeof window === "undefined") return null;
  const raw = window.localStorage.getItem(storageKey);
  if (raw === cachedRaw) return cachedIdentity;
  cachedRaw = raw;
  if (!raw) return null;
  try {
    cachedIdentity = JSON.parse(raw) as Identity;
    return cachedIdentity;
  } catch {
    cachedIdentity = null;
    window.localStorage.removeItem(storageKey);
    return null;
  }
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function notify() {
  listeners.forEach((listener) => listener());
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const identity = useSyncExternalStore(subscribe, readIdentity, () => null);
  const ready = useSyncExternalStore(subscribe, () => true, () => false);
  const value = useMemo(() => ({
    identity,
    ready,
    signIn: (next: Identity) => {
      const raw = JSON.stringify(next);
      cachedRaw = raw;
      cachedIdentity = next;
      window.localStorage.setItem(storageKey, raw);
      notify();
    },
    updateIdentity: (patch: Partial<Identity>) => {
      if (!identity) return;
      const next = { ...identity, ...patch };
      const raw = JSON.stringify(next);
      cachedRaw = raw;
      cachedIdentity = next;
      window.localStorage.setItem(storageKey, raw);
      notify();
    },
    signOut: () => {
      cachedRaw = null;
      cachedIdentity = null;
      window.localStorage.removeItem(storageKey);
      notify();
    },
  }), [identity, ready]);
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used inside AuthProvider");
  return value;
}
