import type { Config } from "tailwindcss";

const config: Config = {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}", "./lib/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        teal: "#0EA5A4",
        navy: "#1E3A5F",
        canvas: "#F4F8FB",
        line: "#E2EBF3",
        amber: "#F59E0B",
        blue: "#3B82F6",
        green: "#22C55E",
        red: "#EF4444",
      },
      boxShadow: {
        card: "0 8px 30px rgba(30, 58, 95, 0.06)",
      },
    },
  },
  plugins: [],
};

export default config;
