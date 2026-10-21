export type Patient = {
  id: string; name: string; dateOfBirth: string; gender: string; phone: string; email?: string;
  createdAt: string; updatedAt?: string;
};
export type RecordItem = {
  id: string; patientId: string; appointmentId: string; doctorId: string; doctorName: string;
  visitDate: string; diagnosis: string; prescription: string; createdAt: string;
};
export type Doctor = {
  id: string; name: string; specialization: string; email: string; phone: string; createdAt: string;
};
export type Slot = {
  id: string; doctorId: string; date: string; startTime: string; status: "AVAILABLE" | "HELD" | "RESERVED";
  appointmentId?: string;
};
export type AppointmentRequest = {
  id: string; appointmentId: string; doctorId: string; patientId: string; patientName: string;
  patientEmail: string; doctorName: string; date: string; time: string; reason: string;
  status: "PENDING" | "ACCEPTED" | "REJECTED"; rejectionReason?: string; createdAt: string; updatedAt: string;
};
export type Appointment = {
  id: string; patientId: string; patientName: string; patientEmail: string; doctorId: string;
  doctorName: string; date: string; time: string; reason: string;
  status: "PENDING" | "CONFIRMED" | "COMPLETED" | "REJECTED"; rejectionReason?: string;
  diagnosis?: string; prescription?: string; notes?: string; createdAt: string; updatedAt: string;
};
export type Notification = {
  id: string; eventId: string; eventType: string; recipientType: string; recipientId: string;
  recipientName: string; recipientEmail: string; channel: string; subject: string; message: string;
  status: string; createdAt: string; sentAt?: string; readAt?: string;
};
