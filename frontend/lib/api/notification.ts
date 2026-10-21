import { serviceUrls } from "../config";
import { Notification } from "../types";
import { request } from "./client";

export const listNotifications = (eventType = "", recipientId = "") => {
  const params = new URLSearchParams();
  if (eventType) params.set("eventType", eventType);
  if (recipientId) params.set("recipientId", recipientId);
  return request<Notification[]>(serviceUrls.notification, `/api/notifications${params.size ? `?${params}` : ""}`);
};
export const markNotificationRead = (id: string) =>
  request<Notification>(serviceUrls.notification, `/api/notifications/${id}/read`, { method: "PATCH" });
export const health = () => request<{ status: string }>(serviceUrls.notification, "/health");
