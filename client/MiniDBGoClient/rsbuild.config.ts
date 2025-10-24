import { defineConfig } from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'
import { pluginSass } from '@rsbuild/plugin-sass'
import { pluginSvgr } from '@rsbuild/plugin-svgr'
import { DefinePlugin } from '@rspack/core'
import path from 'path'

export default defineConfig({
  plugins: [pluginSvgr(), pluginSass(), pluginReact()],
  server: {
    // Chuyển tiếp (proxy) tất cả request /api
    // từ server 3000 (React) sang server 6866 (Go)
    proxy: {
      '/api': {
        target: 'http://localhost:6866',
        changeOrigin: true,
      },
    },
  },
  tools: {
    rspack: {
      output: {
        path: path.resolve(__dirname, 'dist'),
        publicPath: '/',
        filename: '[name].js',
        chunkFilename: '[name].js',
        uniqueName: 'MiniDBGoClient',
      },
      module: {
        rules: [],
      },
      plugins: [
        new DefinePlugin({
          'process.env': JSON.stringify(process.env),
        }),
      ],
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
})
