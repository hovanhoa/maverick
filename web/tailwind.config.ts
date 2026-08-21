import type { Config } from 'tailwindcss';
import defaultTheme from 'tailwindcss/defaultTheme';
import colors from 'tailwindcss/colors';
import forms from '@tailwindcss/forms';

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  darkMode: 'class',
  theme: {
    fontFamily: {
      sans: ['Inter', ...defaultTheme.fontFamily.sans],
      heading: ['Rubik', ...defaultTheme.fontFamily.sans],
      mono: ['ui-monospace', ...defaultTheme.fontFamily.mono]
    },
    extend: {
      colors: {
        primary: colors.sky,
        accent: colors.amber,
        neutral: colors.slate
      }
    }
  },
  plugins: [forms]
} satisfies Config;
