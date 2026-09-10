import type { Config } from 'tailwindcss';

const config: Config = {
  darkMode: ['class'],
  content: [
    './src/**/*.{js,ts,jsx,tsx,mdx}',
    '../../packages/ui/src/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  presets: [require('../../packages/ui/tailwind.config.ts')],
  theme: {
    extend: {
      colors: {
        highlight: {
          DEFAULT: 'hsl(var(--highlight))',
          foreground: 'hsl(var(--highlight-foreground))',
        },
      },
      fontFamily: {
        display: ['var(--font-display)'],
      },
      boxShadow: {
        card: '0 1px 2px rgba(0,0,0,.04), 0 8px 24px -8px rgba(0,0,0,.12)',
      },
    },
  },
};

export default config;
