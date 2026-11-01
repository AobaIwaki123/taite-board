import { defineConfig } from 'vite';
import preact from '@preact/preset-vite';
import tailwindcss from '@tailwindcss/vite';
import path from 'path';
import { readFileSync, existsSync } from 'fs';

// Read version from root VERSION file or fallback to package.json
let appVersion = '1.0.0';
const rootVersionPath = path.resolve(import.meta.dirname, '../../VERSION');
const pkgPath = path.resolve(import.meta.dirname, './package.json');

if (existsSync(rootVersionPath)) {
  appVersion = readFileSync(rootVersionPath, 'utf-8').trim();
} else if (existsSync(pkgPath)) {
  const pkg = JSON.parse(readFileSync(pkgPath, 'utf-8'));
  appVersion = pkg.version || appVersion;
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
  },
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
});
