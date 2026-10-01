// Plugins
import Components from 'unplugin-vue-components/vite'
import Vue from '@vitejs/plugin-vue'
import Vuetify, { transformAssetUrls } from 'vite-plugin-vuetify'
import VueRouter from 'unplugin-vue-router/vite'

// Utilities
import { defineConfig } from 'vite'
import { fileURLToPath, URL } from 'node:url'

// https://vitejs.dev/config/
export default defineConfig({
  base: '/ui',
  plugins: [
    VueRouter(),
    Vue({
      template: { transformAssetUrls },
    }),
    // https://github.com/vuetifyjs/vuetify-loader/tree/master/packages/vite-plugin#readme
    Vuetify({
      autoImport: true,
      styles: {
        configFile: 'src/styles/settings.scss',
      },
    }),
    Components(),
  ],
  build: {
    // remove the results of previous builds from dist, as dist is committed and embedded into the TAF binary
    // (this is the default for an outDir inside the project root, but stays in effect if outDir is ever moved)
    emptyOutDir: true,
  },
  css: {
    // use Sass' modern API instead of its deprecated legacy JS API
    preprocessorOptions: {
      sass: { api: 'modern-compiler' },
      scss: { api: 'modern-compiler' },
    },
  },
  define: { 'process.env': {} },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
    extensions: [
      '.js',
      '.json',
      '.jsx',
      '.mjs',
      '.ts',
      '.tsx',
      '.vue',
    ],
  },
  server: {
    port: 7779,
    proxy: {
      '/ws': {
        target: 'ws://localhost:7778',
        rewriteWsOrigin: true,
        ws: true
      },
      '/api': {
        target: 'http://localhost:7778',
      }
    }
  },
})
