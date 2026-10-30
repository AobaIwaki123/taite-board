/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        rose: {
          400: '#fb7185',
          500: '#f43f5e',
          600: '#e11d48',
          950: '#4c0519',
        },
        ayu: {
          red: '#ef4444',
          darkred: '#7f1d1d',
        }
      },
      fontFamily: {
        sans: ['-apple-system', 'BlinkMacSystemFont', '"Hiragino Sans"', '"Hiragino Kaku Gothic ProN"', '"Noto Sans JP"', 'sans-serif'],
        display: ['-apple-system', 'BlinkMacSystemFont', '"Hiragino Sans"', 'sans-serif'],
      }
    },
  },
  plugins: [],
}
