import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const devPort = parseInt(env.VITE_DEV_PORT || '5174', 10);
  const apiPort = env.API_PORT || '8081';
  const isProduction = mode === 'production';

  return {
    plugins: [react()],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    server: {
      port: devPort,
      proxy: {
        '/api': {
          target: `http://localhost:${apiPort}`,
          changeOrigin: true,
        },
        '/ws': {
          target: `ws://localhost:${apiPort}`,
          ws: true,
        },
      },
    },
    build: {
      outDir: 'dist',
      sourcemap: isProduction ? false : true,
      minify: isProduction ? 'esbuild' : false,
      target: 'es2020',
      // Split chunks for better caching.
      rollupOptions: {
        output: {
          manualChunks: {
            // Vendor chunks for common dependencies.
            'react-vendor': ['react', 'react-dom'],
            'router': ['react-router-dom'],
            'query': ['@tanstack/react-query'],
            'ui': ['@headlessui/react'],
          },
          // Asset file naming for cache busting.
          assetFileNames: (assetInfo) => {
            const info = assetInfo.name ?? '';
            if (/\.(css)$/.test(info)) {
              return 'assets/css/[name]-[hash][extname]';
            }
            if (/\.(woff|woff2|eot|ttf|otf)$/.test(info)) {
              return 'assets/fonts/[name]-[hash][extname]';
            }
            if (/\.(png|jpe?g|gif|svg|webp|ico)$/.test(info)) {
              return 'assets/images/[name]-[hash][extname]';
            }
            return 'assets/[name]-[hash][extname]';
          },
          chunkFileNames: 'assets/js/[name]-[hash].js',
          entryFileNames: 'assets/js/[name]-[hash].js',
        },
      },
      // Chunk size warnings.
      chunkSizeWarningLimit: 500,
    },
    // Optimize dependencies.
    optimizeDeps: {
      include: ['react', 'react-dom', 'react-router-dom', '@tanstack/react-query'],
    },
    // Enable CSS code splitting.
    css: {
      devSourcemap: true,
    },
  };
});
