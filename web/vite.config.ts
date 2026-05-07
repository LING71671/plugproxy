import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vite';
import { resolve } from 'node:path';

export default defineConfig({
  base: '/ui/',
  plugins: [svelte()],
  build: {
    outDir: resolve(__dirname, '../internal/web/dist'),
    emptyOutDir: true,
    sourcemap: false
  },
  server: {
    proxy: {
      '/metrics.json': 'http://127.0.0.1:8899',
      '/refresh': 'http://127.0.0.1:8899'
    }
  },
  test: {
    environment: 'node'
  }
});
