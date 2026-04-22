import path from 'node:path';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  base: '/assets/',
  build: {
    outDir: path.resolve(__dirname, '../internal/static/assets'),
    emptyOutDir: true,
    assetsDir: '',
    cssCodeSplit: false,
  },
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/stats': 'http://127.0.0.1:8080',
      '/refresh': 'http://127.0.0.1:8080',
      '/check-quota': 'http://127.0.0.1:8080',
      '/recover-auth': 'http://127.0.0.1:8080',
      '/oauth': 'http://127.0.0.1:8080',
      '/admin': 'http://127.0.0.1:8080',
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  },
});
