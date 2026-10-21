import { serviceUrls } from "../config";
import { Patient, RecordItem } from "../types";
import { json, request } from "./client";

export const listPatients = () => request<{ data: Patient[] }>(serviceUrls.patient, "/api/patients?limit=100");
export const getPatient = (id: string) => request<Patient>(serviceUrls.patient, `/api/patients/${id}`);
export const createPatient = (body: Omit<Patient, "id" | "createdAt" | "updatedAt">) =>
  request<Patient>(serviceUrls.patient, "/api/patients", json(body));
export const listRecords = (id: string) =>
  request<{ data: RecordItem[] }>(serviceUrls.patient, `/api/patients/${id}/medical-records?limit=100`);
export const health = () => request<{ status: string }>(serviceUrls.patient, "/health");
