import React, { useEffect, useRef, useState } from 'react';

// Props mới: nhận từ App.tsx
interface RequestPanelProps {
  activeCollection: string | null;
  loading: boolean;
  onSend: (method: string, path: string, body: string | null) => void;
}

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE';

const DEFAULT_BODY_SEARCH = '{\n  "group": "vip"\n}';
const DEFAULT_BODY_INSERT_MANY =
  '[\n  {\n    "_id": "doc1",\n    "name": "Alice"\n  },\n  {\n    "_id": "doc2",\n    "name": "Bob"\n  }\n]';
const DEFAULT_BODY_PUT = '{\n  "name": "Updated Name",\n  "new_field": true\n}';

export default function RequestPanel({
  activeCollection,
  loading,
  onSend,
}: RequestPanelProps) {
  const [method, setMethod] = useState<Method>('POST');
  const [path, setPath] = useState('/{collection}/_search');
  const [body, setBody] = useState(DEFAULT_BODY_SEARCH);
  const [activeTab, setActiveTab] = useState('request-body-content');
  const [helperId, setHelperId] = useState('my-doc-id'); // State cho helper ID

  // Tự động cập nhật path khi collection thay đổi
  useEffect(() => {
    if (activeCollection) {
      // Giữ nguyên method và body nếu đang dùng các API đặc biệt
      if (path.endsWith('/_search')) {
        setPath(`/${activeCollection}/_search`);
        setBody(DEFAULT_BODY_SEARCH);
        setMethod('POST');
      } else if (path.endsWith('/_insertMany')) {
        setPath(`/${activeCollection}/_insertMany`);
        setBody(DEFAULT_BODY_INSERT_MANY);
        setMethod('POST');
      } else {
        // Mặc định về _search
        setPath(`/${activeCollection}/_search`);
        setBody(DEFAULT_BODY_SEARCH);
        setMethod('POST');
      }
    } else {
      setPath('/{collection}/_search');
    }
  }, [activeCollection]);

  // Xử lý các nút helper
  const getPath = (id: string) =>
    `/${activeCollection || '{collection}'}/${id}`;

  const handleHelperGet = () => {
    setMethod('GET');
    setPath(getPath(helperId));
    setBody('');
    setActiveTab('request-body-content');
  };

  const handleHelperPut = () => {
    setMethod('PUT');
    setPath(getPath(helperId));
    setBody(DEFAULT_BODY_PUT);
    setActiveTab('request-body-content');
  };

  const handleHelperDelete = () => {
    setMethod('DELETE');
    setPath(getPath(helperId));
    setBody('');
    setActiveTab('request-body-content');
  };

  const handleHelperSearch = () => {
    setMethod('POST');
    setPath(`/${activeCollection || '{collection}'}/_search`);
    setBody(DEFAULT_BODY_SEARCH);
    setActiveTab('request-body-content');
  };

  const handleHelperInsertMany = () => {
    setMethod('POST');
    setPath(`/${activeCollection || '{collection}'}/_insertMany`);
    setBody(DEFAULT_BODY_INSERT_MANY);
    setActiveTab('request-body-content');
  };

  // Xử lý gửi request
  const handleSend = () => {
    const bodyToSend = method === 'POST' || method === 'PUT' ? body : null;
    onSend(method, path, bodyToSend);
  };

  return (
    <div
      id="request-panel"
      className="flex-1 bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden flex flex-col"
    >
      <div className="flex-shrink-0 flex gap-2 p-4 border-b border-gray-200 relative">
        {/* Dropdown đơn giản cho Method */}
        <select
          id="method-toggle-button"
          data-method={method}
          value={method}
          onChange={(e) => setMethod(e.target.value as Method)}
          className="method-btn w-28 text-left px-4 py-2 text-sm font-bold bg-gray-100 border border-gray-300 rounded-md"
        >
          <option value="POST">POST</option>
          <option value="GET">GET</option>
          <option value="PUT">PUT</option>
          <option value="DELETE">DELETE</option>
        </select>

        <input
          value={path}
          onChange={(e) => setPath(e.target.value)}
          type="text"
          id="rest-path"
          className="flex-1 min-w-0 px-4 py-2 text-sm font-mono bg-white border border-gray-300 rounded-md"
          placeholder="/{collection}/{id}"
        />

        <button
          id="btn-send-rest"
          onClick={handleSend} // Thêm onClick
          disabled={loading} // Thêm disabled
          className="flex-shrink-0 flex items-center justify-center gap-2 px-6 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-700 transition-colors disabled:opacity-50"
        >
          <i data-feather="send" className="w-4 h-4" />
          Send
        </button>
      </div>

      <div
        id="request-sub-tabs"
        className="flex-shrink-0 flex border-b border-gray-200 px-4"
      >
        <button
          onClick={() => setActiveTab('request-body-content')}
          className={`sub-tab-button py-3 px-4 text-sm font-medium ${activeTab === 'request-body-content' ? 'active' : ''
            }`}
        >
          Body
        </button>
        <button
          onClick={() => setActiveTab('request-params-content')}
          className={`sub-tab-button py-3 px-4 text-sm font-medium ${activeTab === 'request-params-content' ? 'active' : ''
            }`}
        >
          Query Helpers
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-4">
        <div
          id="request-body-content"
          className={`${activeTab === 'request-body-content' ? '' : 'hidden'}`}
        >
          <textarea
            id="rest-body"
            value={body}
            onChange={(e) => setBody(e.target.value)}
            placeholder='{"key": "value"}'
            className="w-full h-64 p-3 text-sm font-mono bg-gray-50 border border-gray-300 rounded-md resize-vertical"
            disabled={method === 'GET' || method === 'DELETE'}
          />
        </div>
        <div
          id="request-params-content"
          className={`${activeTab === 'request-params-content' ? '' : 'hidden'}`}
        >
          <p className="text-sm text-gray-600 mb-4">
            Sử dụng các nút trợ giúp này để tự động điền vào thanh Request ở trên
            (yêu cầu chọn Collection trước).
          </p>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="p-4 bg-gray-50 rounded-lg border border-gray-200">
              <h3 className="font-semibold text-gray-800 mb-3">
                Thao tác theo ID
              </h3>
              <input
                id="helper-id"
                value={helperId}
                onChange={(e) => setHelperId(e.target.value)}
                className="w-full px-3 py-2 text-sm font-mono bg-white border border-gray-300 rounded-md"
              />
              <div className="flex gap-2 mt-3">
                <button
                  onClick={handleHelperGet}
                  className="flex-1 px-3 py-2 text-sm font-medium text-green-700 bg-green-100 rounded-md"
                >
                  GET
                </button>
                <button
                  onClick={handleHelperPut}
                  className="flex-1 px-3 py-2 text-sm font-medium text-yellow-700 bg-yellow-100 rounded-md"
                >
                  PUT
                </button>
                <button
                  onClick={handleHelperDelete}
                  className="flex-1 px-3 py-2 text-sm font-medium text-red-700 bg-red-100 rounded-md"
                >
                  DELETE
                </button>
              </div>
            </div>
            <div className="p-4 bg-gray-50 rounded-lg border border-gray-200">
              <h3 className="font-semibold text-gray-800 mb-3">
                Thao tác hàng loạt
              </h3>
              <p className="text-xs text-gray-500 mb-3">
                Sử dụng các API `_search` hoặc `_insertMany` cho Collection đã
                chọn.
              </p>
              <div className="flex gap-2 mt-3">
                <button
                  onClick={handleHelperSearch}
                  className="flex-1 px-3 py-2 text-sm font-medium text-blue-700 bg-blue-100 rounded-md"
                >
                  _search
                </button>
                <button
                  onClick={handleHelperInsertMany}
                  className="flex-1 px-3 py-2 text-sm font-medium text-purple-700 bg-purple-100 rounded-md"
                >
                  _insertMany
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}