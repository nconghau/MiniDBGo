import React, { useState } from 'react';
import { ResponseData } from '../data/api';
import ReactJson from 'react-json-view';
import { Rocket, Loader2 } from 'lucide-react';

interface ResponsePanelProps {
  response: ResponseData | null;
  loading: boolean;
}

// --- Component cho trạng thái rỗng (Empty State) ---
const EmptyState = ({ message }: { message: string }) => (
  <div className="flex flex-col items-center justify-center h-full text-center text-slate-500 p-8">
    <Rocket className="w-16 h-16 text-slate-300 mb-4" strokeWidth={1} />
    <h3 className="text-lg font-semibold text-slate-700">Sẵn sàng nhận phản hồi</h3>
    <p className="mt-2 text-sm">{message}</p>
  </div>
);
// --- Component cho trạng thái đang tải (Loading State) ---
const LoadingState = () => (
  <div className="flex flex-col items-center justify-center h-full text-center text-slate-500 p-8">
    <Loader2 className="w-12 h-12 text-primary-600 animate-spin mb-4" strokeWidth={2} />
    <h3 className="text-lg font-semibold text-slate-700">Đang tải...</h3>
    <p className="mt-2 text-sm">Chờ phản hồi từ máy chủ</p>
  </div>
);

export default function ResponsePanel({
  response,
  loading,
}: ResponsePanelProps) {
  const [activeTab, setActiveTab] = useState('response-body-content');

  let recordCount: number | string = '–';
  if (!loading && response && !response.isError) {
    if (Array.isArray(response.body)) {
      recordCount = response.body.length;
    } else if (response.body && typeof response.body === 'object' && Object.keys(response.body).length > 0) {
      recordCount = 1;
    } else if (response.status.startsWith('200') || response.status.startsWith('201')) {
      recordCount = 1;
    } else {
      recordCount = 0;
    }
  } else if (!loading && response?.isError) {
    recordCount = 0;
  }

  return (
    // --- CẬP NHẬT: Bỏ card, thêm border-l (để phân cách) ---
    <div
      id="response-panel"
      className="flex-1 overflow-hidden flex flex-col border-l border-gray-200"
    >
      {/* --- CẬP NHẬT: Nền panel xám nhạt (slate) --- */}
      <div className="flex-shrink-0 flex flex-wrap items-center gap-x-6 gap-y-2 p-4 bg-slate-50 border-b border-gray-200">
        <span className="text-sm">
          Status:{' '}
          <b
            id="response-status"
            className={`font-bold ${response?.isError ? 'text-red-600' : 'text-green-600'
              }`}
          >
            {loading ? '...' : response?.status || '–'}
          </b>
        </span>
        <span className="text-sm">
          Time:{' '}
          <b id="response-time" className="font-bold">
            {loading ? '...' : response?.time || '– ms'}
          </b>
        </span>
        <span className="text-sm">
          Size:{' '}
          <b id="response-size" className="font-bold">
            {loading ? '...' : response?.size || '–'}
          </b>
        </span>
        <span className="text-sm">
          Records:{' '}
          <b id="response-records" className="font-bold">
            {loading ? '...' : recordCount}
          </b>
        </span>
      </div>

      <div
        id="response-sub-tabs"
        className="flex-shrink-0 flex border-b border-gray-200 px-1"
      >
        <button
          onClick={() => setActiveTab('response-body-content')}
          className={`sub-tab-button py-3 px-4 text-sm font-medium focus:outline-none ${activeTab === 'response-body-content' ? 'active' : 'text-slate-600 hover:text-slate-800'
            }`}
        >
          Body
        </button>
        <button
          onClick={() => setActiveTab('response-headers-content')}
          className={`sub-tab-button py-3 px-4 text-sm font-medium focus:outline-none ${activeTab === 'response-headers-content' ? 'active' : 'text-slate-600 hover:text-slate-800'
            }`}
        >
          Headers
        </button>
      </div>

      <div className="flex-1 overflow-auto">
        {/* Tab Body */}
        <div
          id="response-body-content"
          className={`${activeTab === 'response-body-content' ? '' : 'hidden'
            } h-full`}
        >
          {loading && <LoadingState />}
          {!loading && !response && (
            <EmptyState message="API Response sẽ xuất hiện tại đây" />
          )}
          {!loading && response && (
            <div className="json-view-container p-4">
              <ReactJson
                src={response.body}
                collapsed={false}
                displayDataTypes={false}
                name={false}
              />
            </div>
          )}
        </div>
        {/* Tab Headers */}
        <div
          id="response-headers-content"
          className={`${activeTab === 'response-headers-content' ? '' : 'hidden'
            } h-full`}
        >
          {loading && <LoadingState />}
          {!loading && (!response || !response.headers) && (
            <EmptyState message="Response Headers sẽ xuất hiện tại đây" />
          )}
          {!loading && response && response.headers && (
            <div className="json-view-container p-4">
              <ReactJson
                src={response.headers}
                collapsed={true}
                displayDataTypes={false}
                name={false}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}