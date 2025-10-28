import React, { useEffect, useState } from 'react'
import { fetchApi, formatBytes } from '../data/api'
import { RefreshCw, Database, Search, LayoutDashboard } from 'lucide-react'

interface CollectionInfo {
  name: string
  docCount: number
  byteSize: number
}

interface SidebarProps {
  activeCollection: string | null
  setActiveCollection: (collection: string) => void
}

// (16384 -> "16.384")
const intFormatter = new Intl.NumberFormat('vi-VN', {
  minimumFractionDigits: 0,
  maximumFractionDigits: 0,
})

export default function Sidebar({
  activeCollection,
  setActiveCollection,
}: SidebarProps) {
  const [collections, setCollections] = useState<CollectionInfo[]>([])
  const [searchTerm, setSearchTerm] = useState('')

  useEffect(() => {
    loadCollections()
  }, [])

  async function loadCollections() {
    try {
      const res = await fetchApi('GET', '/_collections', null, [], [])
      if (res.isError || !Array.isArray(res.body)) {
        throw new Error(res.error || 'Failed to load collections')
      }

      const sortedCollections = (res.body as CollectionInfo[]).sort((a, b) =>
        a.name.localeCompare(b.name),
      )
      setCollections(sortedCollections)
    } catch (e) {
      console.error(e)
      setCollections([])
    }
  }

  const filteredCollections = collections.filter((col) =>
    col.name.toLowerCase().includes(searchTerm.toLowerCase()),
  )

  return (
    // --- CẬP NHẬT: Đổi nền sang xám (slate) ---
    <aside
      id="sidebar"
      className="flex-shrink-0 fixed md:static inset-y-0 left-0 z-40 flex flex-col w-64 bg-slate-50 border-r border-gray-200 transform -translate-x-full md:translate-x-0 transition-transform duration-300 ease-in-out"
    >
      <div className="flex-shrink-0 h-16 flex items-center justify-between p-4 border-b border-gray-200">
        <h2 className="text-sm font-semibold uppercase text-slate-500 tracking-wider">
          Collections
        </h2>
        <button
          type="button"
          title="Refresh Collections"
          onClick={loadCollections}
          className="p-1 text-slate-500 hover:text-primary-600 rounded-full hover:bg-primary-100 focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-primary-500"
        >
          <RefreshCw className="w-4 h-4" />
        </button>
      </div>

      <div className="p-2 border-b border-gray-200">
        <div className="relative">
          <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
            <Search className="w-4 h-4 text-slate-400" />
          </div>
          <input
            type="text"
            placeholder="Search collections..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            // --- CẬP NHẬT: Dùng border slate, focus-within primary ---
            className="block w-full pl-9 pr-3 py-2 border border-slate-300 rounded-md leading-5 bg-white placeholder-slate-500 focus:outline-none focus:ring-0 focus:border-primary-500 sm:text-sm"
          />
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-2">
        <ul id="collection-list">

          <li
            key="dashboard"
            onClick={() => setActiveCollection('__dashboard__')}
            data-collection-name="dashboard"
            className={`flex items-center gap-3 p-3 text-sm rounded-md cursor-pointer focus:outline-none focus:ring-1 focus:ring-primary-500 ${activeCollection === '__dashboard__'
              ? 'active'
              : 'text-slate-700 hover:bg-slate-200'
              }`}
          >
            <LayoutDashboard
              className={`w-4 h-4 ${activeCollection === '__dashboard__'
                ? 'text-primary-600'
                : 'text-slate-500'
                }`}
            />
            <span className="flex-1 truncate">Dashboard</span>
          </li>

          {filteredCollections.length === 0 && (
            <li className="p-3 text-sm text-slate-500 italic">
              {searchTerm ? 'No matching collections' : '(No collections)'}
            </li>
          )}

          {filteredCollections.map((col) => (
            <li
              key={col.name}
              onClick={() => setActiveCollection(col.name)}
              data-collection-name={col.name}
              // --- CẬP NHẬT: Chuẩn hóa text-sm, active sẽ dùng CSS ---
              className={`flex items-center gap-3 p-3 text-sm rounded-md cursor-pointer focus:outline-none focus:ring-1 focus:ring-primary-500 ${activeCollection === col.name
                ? 'active' // .active (font-semibold) được định nghĩa trong index.css
                : 'text-slate-700 hover:bg-slate-200'
                }`}
            >
              <Database
                className={`w-4 h-4 ${activeCollection === col.name
                  ? 'text-primary-600'
                  : 'text-slate-500'
                  }`}
              />
              <span className="flex-1 truncate">{col.name}</span>

              <div className="flex flex-col items-end">
                <span className="text-xs text-slate-500 font-mono bg-slate-200 rounded-full px-2 py-0.5 leading-none">
                  {intFormatter.format(col.docCount)}
                </span>
                {/* <span className="text-[10px] text-slate-400 font-mono mt-0.5">
                  {formatBytes(col.byteSize)}
                </span> */}
              </div>
            </li>
          ))}
        </ul>
      </div>
    </aside>
  )
}
