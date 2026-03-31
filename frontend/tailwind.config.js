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
          card: "#13131A",
          "card-hover": "#1A1A26",
          border: "#23232F",
          "border-light": "#2E2E3E",
          muted: "#8A8A9A",
        },
      },
      backgroundImage: {
        "grid-pattern":
          "radial-gradient(circle, rgba(153, 69, 255, 0.06) 1px, transparent 1px)",
      },
      backgroundSize: {
        "grid-pattern": "24px 24px",
      },
    },
  },
  plugins: [],
};
