import http from 'k6/http';
import { check, group } from 'k6';
import { uuidv4 } from 'k6/crypto'; // 1. Thay thế cho generate_unique_id

// --- Cấu hình ---
const BASE_URL = 'http://localhost:6866/api';

// 2. Số VUs (Virtual Users) cho mỗi kịch bản, tương đương với CONCURRENCY
const CONCURRENCY_PER_API = 50;

// 3. Thời gian chạy test, thay thế cho 'while true'
const TEST_DURATION = '10m';

// Danh sách collections, tương đương với mảng bash
const POSSIBLE_COLLECTIONS = [
  'orders', 'users', 'products', 'inventory', 'logs', 'reviews', 'sessions', 'carts'
];

// --- Định nghĩa kịch bản (Kịch bản) ---
export const options = {
  // Tổng cộng sẽ có 150 VUs (50 * 3) chạy đồng thời
  scenarios: {
    // Kịch bản 1: Chạy test_health_check
    health_check_scenario: {
      executor: 'constant-vus',       // Duy trì số VUs không đổi
      vus: CONCURRENCY_PER_API,       // Số VUs cho kịch bản này
      duration: TEST_DURATION,        // Thời gian chạy
      exec: 'testHealthCheck',      // Tên hàm JS để chạy
      gracefulStop: '5s',             // Thời gian dọn dẹp sau khi hết giờ
    },
    // Kịch bản 2: Chạy test_insert_one
    insert_one_scenario: {
      executor: 'constant-vus',
      vus: CONCURRENCY_PER_API,
      duration: TEST_DURATION,
      exec: 'testInsertOne',
      gracefulStop: '5s',
    },
    // Kịch bản 3: Chạy test_insert_many
    insert_many_scenario: {
      executor: 'constant-vus',
      vus: CONCURRENCY_PER_API,
      duration: TEST_DURATION,
      exec: 'testInsertMany',
      gracefulStop: '5s',
    },
  },
  // Ngưỡng thất bại: Dừng test nếu hơn 1% request bị lỗi
  thresholds: {
    'http_req_failed': ['rate<0.01'],
  },
};

// --- Hàm trợ giúp ---

// 4. Hàm thay thế cho 'get_random_collection'
function getRandomCollection() {
  return POSSIBLE_COLLECTIONS[Math.floor(Math.random() * POSSIBLE_COLLECTIONS.length)];
}

// [SỬA LỖI 1] Hàm `getUniqueId` mới, thay thế cho k6/crypto
function getUniqueId() {
  // Hàm tạo UUIDv4 bằng JS thuần túy
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    var r = Math.random() * 16 | 0, v = c == 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}

// --- Các hàm Test API (được gọi bởi Scenarios) ---

export function testHealthCheck() {
  group('API: Health Check', function () {
    const res = http.get(`${BASE_URL}/health`);
    
    // [SỬA LỖI 2] Thêm 'r &&'
    check(res, {
      'Health Check: request successful': (r) => r,
      'Health Check: status is 200': (r) => r && r.status === 200,
    });
  });
}

export function testInsertOne() {
  group('API: Insert One', function () {
    const collection = getRandomCollection();
    const id = getUniqueId(); // Bây giờ sẽ dùng hàm mới
    
    const payload = {
      _id: id,
      age: 18,
      category: "electronics",
      city: "Paris",
      country: "eiusmod",
      is_active: true,
      is_verified: true,
      last_login: new Date().toISOString(),
      price: 3770,
      rating: 1,
      tags: ["eco-friendly"],
      title: "k6_test_insert_one"
    };
    
    const params = {
      headers: { 'Content-Type': 'application/json' },
    };
    
    const res = http.post(
      `${BASE_URL}/${collection}`,
      JSON.stringify(payload),
      params
    );
    
    // [SỬA LỖI 2] Thêm 'r &&'
    check(res, {
      'InsertOne: request successful': (r) => r,
      'InsertOne: status is 200 or 201': (r) => r && (r.status === 200 || r.status === 201),
    });
  });
}

export function testInsertMany() {
  group('API: Insert Many', function () {
    const collection = getRandomCollection();
    const id1 = getUniqueId();
    const id2 = getUniqueId();
    
    const payload = [
      {
        _id: id1,
        age: 40,
        category: "electronics",
        city: "Ho Chi Minh",
        country: "eiusmod",
        is_active: true,
        is_verified: false,
        last_login: new Date().toISOString(),
        price: 780,
        rating: 3,
        tags: ["new", "popular", "sale"],
        title: "k6_test_insert_many_1"
      },
      {
        _id: id2,
        age: 92,
        category: "electronics",
        city: "Hai Phong",
        country: "magna",
        is_active: true,
        is_verified: true,
        last_login: new Date().toISOString(),
        price: 225,
        rating: 5,
        tags: ["popular", "eco-friendly", "new"],
        title: "k6_test_insert_many_2"
      }
    ];

    const params = {
      headers: { 'Content-Type': 'application/json' },
    };
    
    const res = http.post(
      `${BASE_URL}/${collection}/_insertMany`,
      JSON.stringify(payload),
      params
    );
    
    // [SỬA LỖI 2] Thêm 'r &&'
    check(res, {
      'InsertMany: request successful': (r) => r,
      'InsertMany: status is 200 or 201': (r) => r && (r.status === 200 || r.status === 201),
    });
  });
}