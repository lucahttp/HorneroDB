/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'sans-serif'],
        mono: ['JetBrains Mono', 'IBM Plex Mono', 'monospace'],
      },
      colors: {
        primary: {
          DEFAULT: '#36C9FF',
          hover: '#1EB8E6',
          light: '#E0F4FF',
        },
        accent: {
          DEFAULT: '#FF6B36',
          hover: '#E65A2A',
          light: '#FFE0D4',
        },
        surface: {
          DEFAULT: '#FAFAFA',
          dark: '#1A1A1A',
        },
        border: {
          DEFAULT: '#222222',
          light: '#E5E5E5',
          dark: '#333333',
        },
      },
      borderWidth: {
        '3': '3px',
      },
      borderRadius: {
        'xl': '12px',
        '2xl': '16px',
        'pill': '999px',
      },
      animation: {
        'spin-slow': 'spin 2s linear infinite',
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' },
        },
      },
    },
  },
  plugins: [],
}
