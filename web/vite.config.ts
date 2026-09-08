import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import { AntDesignVueResolver } from 'unplugin-vue-components/resolvers'
import { fileURLToPath, URL } from 'url'

// The build output goes straight into web/assets/, which the Go binary embeds
// with //go:embed. `npm run build` therefore fully prepares the frontend for
// `go build`.
export default defineConfig({
  define: {
    __BUILT_AT__: JSON.stringify(
      new Date().toISOString().replace('T', ' ').slice(0, 16),
    ),
  },
  plugins: [
    vue(),
    Components({
      dts: 'src/components.d.ts',
      resolvers: [AntDesignVueResolver({ importStyle: false })],
    }),
  ],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  base: '/',
  build: {
    outDir: 'assets',
    emptyOutDir: true,
    target: 'es2020',
    chunkSizeWarningLimit: 1200,
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['vue', 'vue-router', 'pinia'],
          antd: ['ant-design-vue'],
          editor: [
            '@codemirror/state',
            '@codemirror/view',
            '@codemirror/lang-java',
            '@codemirror/commands',
            '@codemirror/autocomplete',
            '@codemirror/search',
            '@codemirror/lint',
            '@codemirror/language',
          ],
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
    },
  },
})
