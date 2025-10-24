import React, { useState } from 'react';
import { ResponseData } from '../data/api';
import ReactJson from 'react-json-view';

interface ResponsePanelProps {
  response: ResponseData | null;
  loading: boolean;
}

// Hàm helper để render placeholder
const renderPlaceholder = (text: string, isHeader = false) => (
  <pre className={isHeader ? 'h-full p-5' : 'h-full'}>
    <code
      id={isHeader ? '' : 'response-body-code'}
      className="language-json h-full"
    >
      {text}
    </code>
  </pre>
);

export default function ResponsePanel({
  response,
  loading,
}: ResponsePanelProps) {
  const [activeTab, setActiveTab] = useState('response-body-content');

  return (
    <div
      id="response-panel"
      className="flex-1 bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden flex flex-col"
    >
      {/* --- THÊM HIỂN THỊ SIZE --- */}
      <div className="flex-shrink-0 flex items-center gap-6 p-4 bg-gray-50 border-b border-gray-200">
        <span className="text-sm">
          Status:{' '}
          <b
            id="response-status"
            className={`font-bold ${response?.isError ? 'text-red-600' : 'text-green-600'
              }`}
          >
            {loading ? '...' : response?.status || '--'}
          </b>
        </span>
        <span className="text-sm">
          Time:{' '}
          <b id="response-time" className="font-bold">
            {loading ? '...' : response?.time || '-- ms'}
          </b>
        </span>
        {/* --- 1. Thêm dòng này --- */}
        <span className="text-sm">
          Size:{' '}
          <b id="response-size" className="font-bold">
            {loading ? '...' : response?.size || '--'}
          </b>
        </span>
        {/* --- KẾT THÚC THÊM --- */}
      </div>

      <div
        id="response-sub-tabs"
        className="flex-shrink-0 flex border-b border-gray-200 px-4"
      >
        <button
          onClick={() => setActiveTab('response-body-content')}
          className={`sub-tab-button py-3 px-4 text-sm font-medium ${activeTab === 'response-body-content' ? 'active' : ''
            }`}
        >
          Body
        </button>
        <button
          onClick={() => setActiveTab('response-headers-content')}
          className={`sub-tab-button py-3 px-4 text-sm font-medium ${activeTab === 'response-headers-content' ? 'active' : ''
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
          {loading && renderPlaceholder('// Loading...')}
          {!loading && !response && renderPlaceholder('// API Response will appear here')}
          {!loading && response && (
            <div className="json-view-container p-5">
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
          {loading && renderPlaceholder('// Loading...', true)}
          {!loading && (!response || !response.headers) &&
            renderPlaceholder('// Response Headers will appear here', true)}
          {!loading && response && response.headers && (
            <div className="json-view-container p-5">
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