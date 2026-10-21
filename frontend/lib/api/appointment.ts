import { serviceUrls } from "../config";
import { Appointment } from "../types";
import { json, request } from "./client";

export const listAppointments = (status = "", patientId = "", doctorId = "") => {
  const params = new URLSearchParams();
  if (status) params.set("status", status);
  if (patientId) params.set("patientId", patientId);
  if (doctorId) params.set("doctorId", doctorId);
  return request<Appointment[]>(serviceUrls.appointment, `/api/appointments${params.size ? `?${params}` : ""}`);
};
export const getAppointment = (id: string) => request<Appointment>(serviceUrls.appointment, `/api/appointments/${id}`);
export const createAppointment = (body: {
  patientId: string; patientName: string; patientEmail: string; doctorId: string; doctorName: string;
  date: string; time: string; reason: string;
}) => request<Appointment>(serviceUrls.appointment, "/api/appointments", json(body));
export const completeAppointment = (id: string, body: { diagnosis: string; prescription: string; notes: string }) =>
  request<Appointment>(serviceUrls.appointment, `/api/appointments/${id}/complete`, { ...json(body), method: "PUT" });
export const health = () => request<{ status: string }>(serviceUrls.appointment, "/health");
