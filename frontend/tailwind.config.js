/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{svelte,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        darkBg: '#0f172a',
        panelBg: '#1e293b',
        accentGlow: '#38bdf8',
      }
    },
  },
  plugins: [],
}
