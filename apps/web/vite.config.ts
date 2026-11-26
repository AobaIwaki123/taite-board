import { defineConfig, type Plugin } from 'vite';
import preact from '@preact/preset-vite';
import tailwindcss from '@tailwindcss/vite';
import path from 'path';
import { execFileSync } from 'child_process';
import { readFileSync, existsSync } from 'fs';
import { parse as parseYaml } from 'yaml';

function systemGraphDevPlugin(): Plugin {
  const repoRoot = path.resolve(import.meta.dirname, '../..');
  const yamlPath = path.resolve(repoRoot, 'docs/system-graph.yaml');
  const genScript = path.resolve(repoRoot, 'scripts/gen-system-graph.mjs');
  const svgRoutes: Record<string, string> = {
    '/__dev/graph-overview.svg': path.resolve(repoRoot, 'docs/graph-overview.svg'),
    '/__dev/graph-dataflow.svg': path.resolve(repoRoot, 'docs/graph-dataflow.svg'),
  };

  return {
    name: 'taite-system-graph-dev',
    apply: 'serve',
    configureServer(server) {
      server.watcher.add(yamlPath);
      server.watcher.on('change', (changedPath) => {
        if (path.resolve(changedPath) === yamlPath) {
          server.ws.send({ type: 'custom', event: 'system-graph-updated' });
        }
      });

      server.middlewares.use((req, res, next) => {
        const urlPath = (req.url || '').split('?')[0];
        if (urlPath === '/__dev/system-graph.json') {
          try {
            const data = parseYaml(readFileSync(yamlPath, 'utf-8'));
            res.setHeader('Content-Type', 'application/json; charset=utf-8');
            res.setHeader('Cache-Control', 'no-store');
            res.end(JSON.stringify(data));
          } catch (err) {
            res.statusCode = 500;
            res.end(JSON.stringify({ error: String(err) }));
          }
          return;
        }
        if (urlPath === '/__dev/render-graph' && req.method === 'POST') {
          try {
            execFileSync(process.execPath, [genScript, '--render'], { cwd: repoRoot, stdio: 'pipe' });
            res.setHeader('Content-Type', 'application/json; charset=utf-8');
            res.end(JSON.stringify({ ok: true }));
          } catch (err) {
            res.statusCode = 500;
            res.end(JSON.stringify({ error: String(err) }));
          }
          return;
        }
        const svgFile = svgRoutes[urlPath];
        if (svgFile && existsSync(svgFile)) {
          res.setHeader('Content-Type', 'image/svg+xml; charset=utf-8');
          res.setHeader('Cache-Control', 'no-store');
          res.end(readFileSync(svgFile, 'utf-8'));
          return;
        }
        next();
      });
    },
  };
}

// Read version from build-time APP_VERSION env or package.json
let appVersion = '1.0.0';
const envAppVersion = (process.env.APP_VERSION || '').trim().replace(/^v/, '');
const pkgPath = path.resolve(import.meta.dirname, './package.json');

if (envAppVersion && envAppVersion !== 'dev') {
  appVersion = envAppVersion;
} else if (existsSync(pkgPath)) {
  const pkg = JSON.parse(readFileSync(pkgPath, 'utf-8'));
  appVersion = (pkg.version || appVersion).replace(/^v/, '');
}

export default defineConfig({
  plugins: [
    tailwindcss(),
    preact(),
    systemGraphDevPlugin(),
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
