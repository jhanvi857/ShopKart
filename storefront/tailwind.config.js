/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    borderRadius: {
      none: '0',
      sm: '2px',
      DEFAULT: '4px',
      md: '4px',
      lg: '4px',
      xl: '4px',
      full: '9999px',
    },
    extend: {
      colors: {
        retail: {
          blue: '#2874f0',
          blueHover: '#1f5ec7',
          yellow: '#ff9f00',
          orange: '#fb641b',
          green: '#388e3c',
          bg: '#f1f3f6',
          dark: '#212121',
          muted: '#878787',
          border: '#e0e0e0',
          surface: '#ffffff',
        },
      },
      fontFamily: {
        sans: ['Inter', '-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'Roboto', 'sans-serif'],
      },
    },
  },
  plugins: [],
};
