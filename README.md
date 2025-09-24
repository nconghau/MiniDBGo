# TinyDB (Mongo-like CLI in Go)

TinyDB là một **mini database** viết bằng **Go**, phục vụ cho mục đích học tập và nghiên cứu cách hoạt động của một database từ bên trong.  
Dự án này không thay thế cho MongoDB/SQLite/Postgres, mà để giúp sinh viên và lập trình viên **hiểu rõ hơn về cơ chế lưu trữ, index, và CLI**.

---

## ✨ Tính năng hiện tại (@Index11)

- **Key-Value Storage** dựa trên file nhị phân (`tiny.db`).
- **Mongo-like CLI** với các lệnh quen thuộc:
  - `insertOne`
  - `findOne`
  - `findMany`
  - `updateOne`
  - `deleteOne`
  - `dumpAll`
  - `exit`
- **Autocomplete** lệnh CLI (Tab).
- **Command history** (↑ ↓) và chỉnh sửa lệnh (← →).
- **Màu sắc CLI** để dễ nhìn hơn.
- **Xuất dữ liệu** ra file JSON `collection_dump_HH-MM_DD-MM-YYYY.json`.

---
