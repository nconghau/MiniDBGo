import { defineConfig } from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'
import { pluginSass } from '@rsbuild/plugin-sass'
import { pluginSvgr } from '@rsbuild/plugin-svgr'
import path from 'path'

export default defineConfig({
  plugins: [pluginSvgr(), pluginSass(), pluginReact()],
  server: {
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
      ],
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
})