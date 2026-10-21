import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { Shell } from "@/components/Shell";
import { AuthProvider } from "@/components/AuthContext";
import "./globals.css";

const inter = Inter({ subsets: ["latin"] });
export const metadata: Metadata = { title: "MediCare HMS", description: "Hospital Management System dashboard" };
export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body className={inter.className}><AuthProvider><Shell>{children}</Shell></AuthProvider></body></html>;
}
