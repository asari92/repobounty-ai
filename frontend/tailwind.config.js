/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        solana: {
          purple: "#9945FF",
          green: "#14F195",
          dark: "#0E0E10",
          card: "#1A1A2E",
          border: "#2A2A3E",
        },
      },
    },
  },
  plugins: [],
};
