import { serviceUrls } from "../config";
import { AppointmentRequest, Doctor, Slot } from "../types";
import { json, request } from "./client";

export const listDoctors = () => request<Doctor[]>(serviceUrls.doctor, "/api/doctors");
export const createDoctor = (body: Omit<Doctor, "id" | "createdAt">) =>
  request<Doctor>(serviceUrls.doctor, "/api/doctors", json(body));
export const listSlots = (doctorId: string, status?: string) =>
  request<Slot[]>(serviceUrls.doctor, `/api/doctors/${doctorId}/slots${status ? `?status=${status}` : ""}`);
export const createSlot = (doctorId: string, body: { date: string; startTime: string }) =>
  request<Slot>(serviceUrls.doctor, `/api/doctors/${doctorId}/slots`, json(body));
export const health = () => request<{ status: string }>(serviceUrls.doctor, "/health");
export const listRequests = (status = "PENDING", doctorId = "") =>
  request<AppointmentRequest[]>(serviceUrls.doctor, `/api/requests?status=${status}${doctorId ? `&doctorId=${doctorId}` : ""}`);
export const acceptRequest = (id: string) =>
  request<AppointmentRequest>(serviceUrls.doctor, `/api/requests/${id}/accept`, { method: "POST" });
export const rejectRequest = (id: string, reason: string) =>
  request<AppointmentRequest>(serviceUrls.doctor, `/api/requests/${id}/reject`, json({ reason }));
