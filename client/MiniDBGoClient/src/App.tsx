import React, { useState } from 'react';
import Sidebar from './components/Sidebar';
import RequestPanel from './components/RequestPanel';
import ResponsePanel from './components/ResponsePanel';
import { fetchApi, ResponseData } from './data/api';

export default function App() {
  // State được nâng lên App
  const [activeCollection, setActiveCollection] = useState<string | null>(null);
  const [response, setResponse] = useState<ResponseData | null>(null);
  const [loading, setLoading] = useState(false);

  // Xử lý gửi request từ RequestPanel
  const handleSendRequest = async (
    method: string,
    path: string,
    body: string | null,
  ) => {
    setLoading(true);
    // Đảm bảo path luôn bắt đầu bằng /api
    const apiPath = path.startsWith('/api') ? path : `/api${path}`;
    const res = await fetchApi(method, apiPath, body);
    setResponse(res);
    setLoading(false);
  };

  // Xử lý nút Compact DB
  const handleCompact = async () => {
    setLoading(true);
    const res = await fetchApi('POST', '/api/_compact', null);
    setResponse(res);
    setLoading(false);
  };

  return (
    <div className="flex flex-col h-screen bg-gray-100 text-gray-900 overflow-hidden">
      <header className="flex-shrink-0 bg-white border-b border-gray-200">
        <div className="h-16 flex items-center justify-between px-4 max-w-full mx-auto">
          <div className="flex items-center gap-4">
            <button
              id="btn-toggle-sidebar"
              className="md:hidden p-1 text-gray-600 hover:text-gray-900"
            >
              <i data-feather="menu" className="w-6 h-6" />
            </button>
            <h1 className="text-xl font-bold text-gray-800">MiniDBGo Client</h1>
          </div>
          <button
            id="btn-compact"
            onClick={handleCompact} // Thêm onClick
            disabled={loading} // Thêm disabled
            className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-red-600 rounded-md hover:bg-red-700 transition-colors disabled:opacity-50"
          >
            <i data-feather="database" className="w-4 h-4" />
            Compact DB
          </button>
        </div>
      </header>

      <div className="flex-1 flex overflow-hidden max-w-full mx-auto w-full">
        {/* Truyền state và hàm handler xuống component con */}
        <Sidebar
          activeCollection={activeCollection}
          setActiveCollection={setActiveCollection}
        />
        <main className="flex-1 flex flex-row overflow-hidden p-4 gap-4">
          <RequestPanel
            activeCollection={activeCollection}
            onSend={handleSendRequest}
            loading={loading}
          />
          <ResponsePanel response={response} loading={loading} />
        </main>
      </div>
    </div>
  );
}