// Định nghĩa kiểu dữ liệu cho phản hồi
export interface ResponseData {
  status: string;
  time: string;
  body: any; // Nội dung JSON đã parse
  headers: Record<string, string>;
  isError: boolean;
  error?: string; // Thông báo lỗi
}

// Hàm gọi API trung tâm
export async function fetchApi(
  method: string,
  path: string, // Đường dẫn đã bao gồm /api/
  body: string | null,
): Promise<ResponseData> {
  const startTime = performance.now();

  const options: RequestInit = {
    method,
    headers: {
      'Content-Type': 'application/json',
      'Accept': 'application/json',
    },
  };

  if (body && (method === 'POST' || method === 'PUT')) {
    options.body = body;
  }

  try {
    const res = await fetch(path, options);
    const endTime = performance.now();
    const time = (endTime - startTime).toFixed(2);

    // Lấy headers
    const responseHeaders: Record<string, string> = {};
    res.headers.forEach((value, key) => {
      responseHeaders[key] = value;
    });

    // Lấy nội dung body
    const textBody = await res.text();
    let jsonBody: any;
    try {
      jsonBody = JSON.parse(textBody);
    } catch (e) {
      jsonBody = textBody; // Nếu không phải JSON, trả về text
    }

    if (!res.ok) {
      // Nếu server trả về lỗi (vd: 404, 500)
      const errorMsg = typeof jsonBody.error === 'string' ? jsonBody.error : textBody;
      return {
        status: `${res.status} ${res.statusText}`,
        time: `${time} ms`,
        body: jsonBody,
        headers: responseHeaders,
        isError: true,
        error: errorMsg,
      };
    }

    // Thành công
    return {
      status: `${res.status} ${res.statusText}`,
      time: `${time} ms`,
      body: jsonBody,
      headers: responseHeaders,
      isError: false,
    };

  } catch (err: any) {
    // Lỗi mạng hoặc lỗi fetch
    const endTime = performance.now();
    const time = (endTime - startTime).toFixed(2);
    return {
      status: 'Network Error',
      time: `${time} ms`,
      body: { error: err.message },
      headers: {},
      isError: true,
      error: err.message,
    };
  }
}