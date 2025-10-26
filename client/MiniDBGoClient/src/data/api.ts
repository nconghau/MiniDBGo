// --- 1. Thêm hàm formatBytes (Giữ nguyên) ---
export function formatBytes(bytes: number, decimals = 2): string {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i]
}

// --- 2. Định nghĩa kiểu KeyValue (MỚI) ---
export interface KeyValueItem {
  id: string // Dùng cho React key
  key: string
  value: string
  enabled: boolean
}

// Định nghĩa kiểu dữ liệu cho phản hồi (Đã cập nhật)
export interface ResponseData {
  status: string
  time: string
  size: string
  body: any
  headers: Record<string, string>
  isError: boolean
  error?: string
}

// --- 3. Cập nhật hàm fetchApi (QUAN TRỌNG) ---
export async function fetchApi(
  method: string,
  path: string, // Đường dẫn CHƯA có /api/ (ví dụ: /_collections)
  body: string | null,
  params: KeyValueItem[], // <-- MỚI
  headers: KeyValueItem[], // <-- MỚI
): Promise<ResponseData> {
  const startTime = performance.now()

  // --- Lấy BASE_URL từ Environment Variables ---
  // Biến này được inject bởi rsbuild từ file .env
  const BASE_URL = import.meta.env.PUBLIC_BASE_URL

  if (!BASE_URL) {
    console.error('LỖI CẤU HÌNH: PUBLIC_BASE_URL chưa được định nghĩa trong file .env')
    // Trả về lỗi ngay lập tức
    return {
      status: 'Config Error',
      time: '0 ms',
      size: '0 Bytes',
      body: { error: 'PUBLIC_BASE_URL is not defined. Please check your .env file.' },
      headers: {},
      isError: true,
      error: 'PUBLIC_BASE_URL is not defined.',
    }
  }

  // --- Xử lý Params ---
  const searchParams = new URLSearchParams()
  params
    .filter((p) => p.enabled && p.key) // Chỉ lấy param
    .forEach((p) => searchParams.append(p.key, p.value))

  const queryString = searchParams.toString()

  // --- THAY ĐỔI CHÍNH: Xây dựng finalPath từ BASE_URL ---
  // `path` của bạn đã bao gồm dấu "/" (ví dụ: "/_compact")
  // `BASE_URL` là "http://localhost:6866/api"
  // Kết quả: "http://localhost:6866/api/_compact?..."
  const finalPath = `${BASE_URL}${path}${queryString ? `?${queryString}` : ''}`

  // --- Xử lý Headers ---
  const requestHeaders: Record<string, string> = {
    // ... (phần còn lại của hàm giữ nguyên)
    // ...
    'Content-Type': 'application/json',
    Accept: 'application/json',
  }
  headers
    .filter((h) => h.enabled && h.key) // Chỉ lấy header
    .forEach((h) => {
      requestHeaders[h.key] = h.value
    })

  const options: RequestInit = {
    method,
    headers: requestHeaders, // <-- Dùng header đã merge
  }

  if (body && (method === 'POST' || method === 'PUT')) {
    options.body = body // [cite: 254]
  }

  try {
    const res = await fetch(finalPath, options) // <-- Dùng finalPath
    // ... (phần còn lại của hàm try...catch giữ nguyên)
    // ...
    const endTime = performance.now()
    const time = (endTime - startTime).toFixed(2)

    const responseHeaders: Record<string, string> = {}
    res.headers.forEach((value, key) => {
      responseHeaders[key] = value
    })

    const textBody = await res.text()
    const sizeInBytes = new TextEncoder().encode(textBody).length
    const size = formatBytes(sizeInBytes)

    let jsonBody: any
    try {
      jsonBody = JSON.parse(textBody) // [cite: 255]
    } catch (e) {
      jsonBody = textBody
    }

    if (!res.ok) {
      const errorMsg =
        typeof jsonBody.error === 'string' ? jsonBody.error : textBody // [cite: 256]
      return {
        status: `${res.status} ${res.statusText}`,
        time: `${time} ms`,
        size: size,
        body: jsonBody,
        headers: responseHeaders,
        isError: true,
        error: errorMsg,
      }
    }

    // Thành công
    return {
      status: `${res.status} ${res.statusText}`, // [cite: 257]
      time: `${time} ms`,
      size: size,
      body: jsonBody,
      headers: responseHeaders,
      isError: false,
    }
  } catch (err: any) {
    // Lỗi mạng hoặc lỗi fetch
    const endTime = performance.now()
    const time = (endTime - startTime).toFixed(2)
    return {
      status: 'Network Error',
      time: `${time} ms`,
      size: '0 Bytes', // [cite: 258]
      body: { error: err.message },
      headers: {},
      isError: true,
      error: err.message,
    }
  }
}
