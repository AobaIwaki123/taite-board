import { defineConfig } from 'vite';
import preact from '@preact/preset-vite';
import tailwindcss from '@tailwindcss/vite';
import path from 'path';
import { readFileSync, existsSync } from 'fs';

// Read version from build-time APP_VERSION env, root VERSION file, or package.json
let appVersion = '1.0.0';
const envAppVersion = (process.env.APP_VERSION || '').trim().replace(/^v/, '');
const rootVersionPath = path.resolve(import.meta.dirname, '../../VERSION');
const pkgPath = path.resolve(import.meta.dirname, './package.json');

if (envAppVersion && envAppVersion !== 'dev') {
  appVersion = envAppVersion;
} else if (existsSync(rootVersionPath)) {
  appVersion = readFileSync(rootVersionPath, 'utf-8').trim().replace(/^v/, '');
} else if (existsSync(pkgPath)) {
  const pkg = JSON.parse(readFileSync(pkgPath, 'utf-8'));
  appVersion = (pkg.version || appVersion).replace(/^v/, '');
}

export default defineConfig({
  plugins: [
    tailwindcss(),
    preact(),
  ],
  define: {
    __APP_VERSION__: JSON.stringify(`v${appVersion}`),
  },
  base: './',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 8089,
    host: true,
    proxy: {
      '/api/admin': {
        target: 'http://127.0.0.1:8082',
        changeOrigin: true,
        rewrite: (p) => p.replace(/^\/api\/admin/, '/api'),
      },
      '/api/sync': {
        target: 'http://127.0.0.1:8083',
        changeOrigin: true,
      },
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
});
