
-----

# TinyDB: A Lightweight, Mongo-like Database in Go

## Overview

TinyDB is a lightweight, educational database engine written in Go, inspired by MongoDB. This project is designed as a learning resource to understand core database internals such as CRUD operations, durability through Write-Ahead Logs (WAL), and the storage architecture of a Log-Structured Merge-Tree (LSM-Tree).

## Core Architecture: How It Works (LSM-Tree)

TinyDB is built upon a **Log-Structured Merge-Tree (LSM-Tree)**, an architecture optimized for high write throughput. Here’s a simple breakdown of how it operates:

### ✍️ The Write Path (Handling New Data)

When you insert or update data (`insertOne`, `updateOne`):

1.  **Safety First (WAL)**: The data is immediately written to a **Write-Ahead Log** (`wal.log`) on disk. This acts as a journal, ensuring that no data is lost even if the database crashes.
2.  **Speed in Memory (MemTable)**: The data is then placed into an in-memory data structure called a **MemTable**, which is a sorted SkipList. Writing to memory is extremely fast.
3.  **Flushing to Disk (SSTable)**: When the MemTable grows to a certain size, it is "frozen" (becoming an *Immutable MemTable*) and its sorted data is flushed to a new, read-only file on disk called an **SSTable** (`.sst` file).

### 🔍 The Read Path (Finding Your Data)

When you fetch data (`findOne`, `findMany`), TinyDB searches for the key in a specific order to ensure the most recent data is found first:

1.  **Check the MemTable**: The active MemTable is checked first, as it contains the very latest writes.
2.  **Check Immutable MemTables**: Any "frozen" MemTables that are waiting to be flushed to disk are checked next.
3.  **Scan SSTables on Disk**: Finally, the database searches the SSTable files on disk, starting from the newest and going to the oldest. A deleted key is marked with a "Tombstone" to indicate it should be ignored.

### ⚙️ Background Maintenance (Compaction)

Over time, many small SSTable files can be created. The `compact` command triggers a **Compaction** process, which merges multiple smaller SSTables into a single, larger one. This process cleans up old or deleted data and optimizes the structure for faster reads.

## ✨ Features

  * **Mongo-like CLI**: An interactive command-line interface with familiar commands.
      * `insertOne`, `findOne`, `findMany`
      * `updateOne` (with `$set` operator), `deleteOne`
      * `dumpDB`, `restoreDB`, `compact`
  * **Durability**: Data safety is ensured with a Write-Ahead Log (WAL).
  * **LSM Storage Engine**: The core is a simple but effective LSM-Tree implementation.
  * **CLI Autocomplete**: Smart command and collection name completion for a better user experience.

## 🚀 Quick Start

### 1\. Clone and Run

```bash
git clone https://github.com/your-username/tinydb
cd tinydb
go run ./cmd/tinydb
```

### 2\. Example Usage

```bash
TinyDB CLI (Mongo-like, @Index20)
Commands: insertOne, findOne, findMany, updateOne, deleteOne, dumpAll, dumpDB, restoreDB, compact, exit

> insertOne customers {"_id":"c1","name":"Alice","group":"vip"}
> findOne customers {"_id":"c1"}
> findMany customers {"group":"vip"}
```

## ⚠️ Disclaimer

This project is for **educational purposes only** and is not production-ready. It is intended as a tool for learning database internals in Go.

-----

-----

# TinyDB: Cơ sở dữ liệu gọn nhẹ, kiểu MongoDB, viết bằng Go

## Tổng quan

TinyDB là một engine cơ sở dữ liệu gọn nhẹ cho mục đích học tập, được viết bằng Go và lấy cảm hứng từ MongoDB. Dự án này được thiết kế như một tài nguyên học hỏi để hiểu về các thành phần cốt lõi bên trong một database, ví dụ như các hoạt động CRUD, cơ chế đảm bảo an toàn dữ liệu qua Write-Ahead Logs (WAL), và kiến trúc lưu trữ của Log-Structured Merge-Tree (LSM-Tree).

## Kiến trúc Lõi: Cách Hoạt động (LSM-Tree)

TinyDB được xây dựng dựa trên kiến trúc **Log-Structured Merge-Tree (LSM-Tree)**, một kiến trúc được tối ưu cho việc ghi dữ liệu với thông lượng cao. Dưới đây là mô tả đơn giản về cách nó vận hành:

### ✍️ Luồng Ghi Dữ liệu (Khi có dữ liệu mới)

Khi bạn chèn hoặc cập nhật dữ liệu (`insertOne`, `updateOne`):

1.  **An toàn là trên hết (WAL)**: Dữ liệu ngay lập tức được ghi vào một file **Write-Ahead Log** (`wal.log`) trên đĩa. File này hoạt động như một cuốn nhật ký, đảm bảo không có dữ liệu nào bị mất ngay cả khi database bị sập.
2.  **Tốc độ trong Bộ nhớ (MemTable)**: Dữ liệu sau đó được đặt vào một cấu trúc trong bộ nhớ (RAM) gọi là **MemTable**, vốn là một SkipList đã được sắp xếp. Việc ghi vào bộ nhớ cực kỳ nhanh.
3.  **Đưa xuống Đĩa (SSTable)**: Khi MemTable đạt đến một kích thước nhất định, nó sẽ bị "đóng băng" (trở thành *Immutable MemTable*) và dữ liệu đã được sắp xếp của nó sẽ được đẩy xuống một file chỉ đọc mới trên đĩa gọi là **SSTable** (file `.sst`).

### 🔍 Luồng Đọc Dữ liệu (Khi tìm kiếm)

Khi bạn lấy dữ liệu (`findOne`, `findMany`), TinyDB sẽ tìm kiếm key theo một thứ tự cụ thể để đảm bảo dữ liệu mới nhất luôn được tìm thấy đầu tiên:

1.  **Kiểm tra MemTable**: MemTable đang hoạt động sẽ được kiểm tra trước tiên, vì nó chứa các bản ghi mới nhất.
2.  **Kiểm tra Immutable MemTables**: Bất kỳ MemTable nào đã bị "đóng băng" và đang chờ được ghi xuống đĩa sẽ được kiểm tra tiếp theo.
3.  **Quét các SSTable trên Đĩa**: Cuối cùng, database sẽ tìm kiếm trong các file SSTable trên đĩa, bắt đầu từ file mới nhất đến file cũ nhất. Một key đã bị xóa sẽ được đánh dấu bằng "Tombstone" để cho biết nó nên được bỏ qua.

### ⚙️ Bảo trì Ngầm (Compaction)

Theo thời gian, sẽ có nhiều file SSTable nhỏ được tạo ra. Lệnh `compact` sẽ kích hoạt một quy trình **Compaction**, gộp nhiều file SSTable nhỏ thành một file lớn duy nhất. Quá trình này giúp dọn dẹp dữ liệu cũ hoặc đã bị xóa và tối ưu hóa cấu trúc để đọc nhanh hơn.

## ✨ Tính năng

  * **CLI kiểu MongoDB**: Giao diện dòng lệnh tương tác với các lệnh quen thuộc.
      * `insertOne`, `findOne`, `findMany`
      * `updateOne` (với toán tử `$set`), `deleteOne`
      * `dumpDB`, `restoreDB`, `compact`
  * **Độ bền (Durability)**: An toàn dữ liệu được đảm bảo với Write-Ahead Log (WAL).
  * **Engine Lưu trữ LSM**: Phần lõi là một hiện thực LSM-Tree đơn giản nhưng hiệu quả.
  * **Tự động hoàn thành lệnh (Autocomplete)**: Gợi ý thông minh tên lệnh và tên collection để cải thiện trải nghiệm người dùng.

## 🚀 Bắt đầu Nhanh

### 1\. Tải về và Chạy

```bash
git clone https://github.com/your-username/tinydb
cd tinydb
go run ./cmd/tinydb
```

### 2\. Ví dụ Sử dụng

```bash
TinyDB CLI (Mongo-like, @Index20)
Commands: insertOne, findOne, findMany, updateOne, deleteOne, dumpAll, dumpDB, restoreDB, compact, exit

> insertOne customers {"_id":"c1","name":"Alice","group":"vip"}
> findOne customers {"_id":"c1"}
> findMany customers {"group":"vip"}
```

## ⚠️ Khuyến cáo

Dự án này chỉ dành cho **mục đích học tập** và chưa sẵn sàng để sử dụng trong môi trường thực tế (production). Nó được tạo ra như một công cụ để tìm hiểu về các thành phần bên trong của một database bằng Go.