import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    deps: {
      interopDefault: true,
    },
  },
  resolve: {
    alias: {
      react: path.resolve('./node_modules/react'),
      'react/jsx-runtime': path.resolve('./node_modules/react/jsx-runtime'),
      'react-dom': path.resolve('./node_modules/react-dom'),
    },
  },
});
