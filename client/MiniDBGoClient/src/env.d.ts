/// <reference types="rsbuild/types" />

interface ImportMetaEnv {
  readonly PUBLIC_BASE_URL: string
  // 👉 Add other variables here as needed
  // readonly API_URL: string
  // readonly APP_ENV: 'development' | 'production'
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
