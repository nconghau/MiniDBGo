import React, { useEffect, useState } from 'react';
import { fetchApi } from '../data/api';

// Props mới: nhận từ App.tsx
interface SidebarProps {
  activeCollection: string | null;
  setActiveCollection: (collection: string) => void;
}

export default function Sidebar({
  activeCollection,
  setActiveCollection,
}: SidebarProps) {
  const [collections, setCollections] = useState<string[]>([]);
  // Đã xóa local state 'active', dùng 'activeCollection' từ props

  useEffect(() => {
    loadCollections();
  }, []);

  async function loadCollections() {
    try {
      // Sử dụng fetchApi
      const res = await fetchApi('GET', '/api/_collections', null);
      if (res.isError || !Array.isArray(res.body)) {
        throw new Error(res.error || 'Failed to load collections');
      }
      setCollections((res.body || []).sort());
    } catch (e) {
      console.error(e);
      setCollections([]);
    }
  }

  return (
    <aside
      id="sidebar"
      className="flex-shrink-0 fixed md:static inset-y-0 left-0 z-40 flex flex-col w-64 bg-gray-50 border-r border-gray-200 transform -translate-x-full md:translate-x-0 transition-transform duration-300 ease-in-out"
    >
      <div className="flex-shrink-0 h-16 flex items-center justify-between p-4 border-b border-gray-200">
        <h2 className="text-sm font-semibold uppercase text-gray-500 tracking-wider">
          Collections
        </h2>
        <button
          title="Refresh Collections"
          onClick={loadCollections}
          className="p-1 text-gray-500 hover:text-blue-600 rounded-full hover:bg-gray-200"
        >
          <i data-feather="refresh-cw" className="w-4 h-4" />
        </button>
      </div>
      <div className="flex-1 overflow-y-auto p-2">
        <ul id="collection-list">
          {collections.length === 0 && (
            <li className="p-3 text-sm text-gray-500 italic">
              (No collections)
            </li>
          )}
          {collections.map((col) => (
            <li
              key={col}
              onClick={() => setActiveCollection(col)} // Cập nhật state của App
              data-collection-name={col}
              className={`flex items-center gap-3 p-3 text-sm text-gray-700 hover:bg-gray-200 cursor-pointer rounded-md ${activeCollection === col ? 'active' : '' // Sử dụng props
                }`}
            >
              <i data-feather="database" className="w-4 h-4 text-gray-500" />
              <span className="flex-1 truncate">{col}</span>
            </li>
          ))}
        </ul>
      </div>
    </aside>
  );
}