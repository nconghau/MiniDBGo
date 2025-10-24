import React, { useEffect, useState } from 'react';
import { Send, Plus, Trash2 } from 'lucide-react';
import Editor from 'react-simple-code-editor';
import { highlight, languages } from 'prismjs';
import 'prismjs/components/prism-json';
import 'prismjs/themes/prism-tomorrow.css';
import { KeyValueItem } from '../data/api';

// (Component KeyValueEditor giữ nguyên, không thay đổi)
interface KeyValueEditorProps {
  items: KeyValueItem[];
  onChange: (items: KeyValueItem[]) => void;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
}
function KeyValueEditor({
  items,
  onChange,
  keyPlaceholder = 'Key',
  valuePlaceholder = 'Value',
}: KeyValueEditorProps) {
  const handleUpdate = (id: string, field: 'key' | 'value' | 'enabled', value: string | boolean) => {
    const newItems = items.map((item) =>
      item.id === id ? { ...item, [field]: value } : item
    );
    onChange(newItems);
  };
  const handleRemove = (id: string) => {
    const newItems = items.filter((item) => item.id !== id);
    onChange(newItems);
  };
  const handleAdd = () => {
    const newItem: KeyValueItem = {
      id: crypto.randomUUID(),
      key: '',
      value: '',
      enabled: true,
    };
    onChange([...items, newItem]);
  };
  return (
    <div className="flex flex-col gap-2">
      {items.map((item) => (
        <div key={item.id} className="flex items-center gap-2">
          <input
            type="checkbox"
            className="form-checkbox h-5 w-5 text-blue-600 rounded"
            checked={item.enabled}
            onChange={(e) => handleUpdate(item.id, 'enabled', e.target.checked)}
          />
          <input
            type="text"
            className="flex-1 min-w-0 px-3 py-2 text-sm font-mono bg-white border border-gray-300 rounded-md"
            placeholder={keyPlaceholder}
            value={item.key}
            onChange={(e) => handleUpdate(item.id, 'key', e.target.value)}
          />
          <input
            type="text"
            className="flex-1 min-w-0 px-3 py-2 text-sm font-mono bg-white border border-gray-300 rounded-md"
            placeholder={valuePlaceholder}
            value={item.value}
            onChange={(e) => handleUpdate(item.id, 'value', e.target.value)}
          />
          <button
            onClick={() => handleRemove(item.id)}
            className="p-2 text-gray-500 hover:text-red-600"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      ))}
      <button
        onClick={handleAdd}
        className="flex items-center gap-2 px-3 py-2 text-sm font-medium text-blue-600 hover:bg-blue-50 rounded-md"
      >
        <Plus className="w-4 h-4" />
        Add
      </button>
    </div>
  );
}
// --- Hết Component KeyValueEditor ---


interface RequestPanelProps {
  activeCollection: string | null;
  loading: boolean;
  onSend: (
    method: string,
    path: string,
    body: string | null,
    params: KeyValueItem[],
    headers: KeyValueItem[]
  ) => void;
}

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE';

const DEFAULT_BODY_SEARCH = '{\n  "category": "electronics"\n}';
const DEFAULT_BODY_INSERT_MANY =
  '[\n  {\n    "_id": "p2",\n    "name": "Mouse"\n  },\n  {\n    "_id": "p3",\n    "name": "Keyboard"\n  }\n]';
const DEFAULT_BODY_PUT = '{\n  "name": "Laptop Pro",\n  "price": 1499\n}';

export default function RequestPanel({
  activeCollection,
  loading,
  onSend,
}: RequestPanelProps) {
  const [method, setMethod] = useState<Method>('POST');
  const [path, setPath] = useState('/{collection}/_search');
  const [body, setBody] = useState(DEFAULT_BODY_SEARCH);
  const [helperId, setHelperId] = useState('p1');

  type RequestTab = 'params' | 'body' | 'headers' | 'suggestions';
  const [activeTab, setActiveTab] = useState<RequestTab>('body');
  const [params, setParams] = useState<KeyValueItem[]>([]);
  const [headers, setHeaders] = useState<KeyValueItem[]>([]);

  useEffect(() => {
    // Chỉ cập nhật path nếu người dùng CHƯA chọn collection
    // (để tránh ghi đè lên path người dùng tự gõ)
    if (!activeCollection) {
      setPath('/{collection}/_search');
    } else {
      // Nếu path vẫn là placeholder, cập nhật nó
      if (path.startsWith('/{collection}')) {
        const newPath = path.replace('/{collection}', `/${activeCollection}`);
        setPath(newPath);
      }
    }
    // Bỏ 'path' khỏi dependency array để không bị vòng lặp
  }, [activeCollection]);

  const getPath = (id: string) =>
    `/${activeCollection || '{collection}'}/${id}`;

  const handleHelperGet = () => {
    setMethod('GET');
    setPath(getPath(helperId));
    setBody('');
    setActiveTab('body');
  };
  const handleHelperPut = () => {
    setMethod('PUT');
    setPath(getPath(helperId));
    setBody(DEFAULT_BODY_PUT);
    setActiveTab('body');
  };
  const handleHelperDelete = () => {
    setMethod('DELETE');
    setPath(getPath(helperId));
    setBody('');
    setActiveTab('body');
  };
  const handleHelperSearch = () => {
    setMethod('POST');
    setPath(`/${activeCollection || '{collection}'}/_search`);
    setBody(DEFAULT_BODY_SEARCH);
    setActiveTab('body');
  };
  const handleHelperInsertMany = () => {
    setMethod('POST');
    setPath(`/${activeCollection || '{collection}'}/_insertMany`);
    setBody(DEFAULT_BODY_INSERT_MANY);
    setActiveTab('body');
  };

  const handleSend = () => {
    const bodyToSend = (method === 'POST' || method === 'PUT') ? body : null;
    onSend(method, path, bodyToSend, params, headers);
  };

  // --- LOGIC VÔ HIỆU HÓA NÚT SEND ---
  // Vô hiệu hóa nếu:
  // 1. Đang loading
  // 2. Path chứa "{collection}" (nghĩa là chưa chọn collection)
  const isSendDisabled = loading || path.includes('{collection}');
  // ---

  return (
    <div
      id="request-panel"
      className="flex-1 bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden flex flex-col"
    >
      <div className="flex-shrink-0 flex gap-2 p-4 border-b border-gray-200 relative">
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
          onClick={handleSend}
          disabled={isSendDisabled} // <-- SỬ DỤNG LOGIC MỚI
          className="flex-shrink-0 flex items-center justify-center gap-2 px-6 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-700 transition-colors disabled:opacity-50"
        >
          <Send className="w-4 h-4" />
          Send
        </button>
      </div>

      {/* (Phần còn lại của file: Tabs, Editor, KeyValueEditor... giữ nguyên) */}
      <div
        id="request-sub-tabs"
        className="flex-shrink-0 flex border-b border-gray-200 px-4"
      >
        <button
          onClick={() => setActiveTab('params')}
          className={`sub-tab-button py-3 px-4 text-sm font-medium ${activeTab === 'params' ? 'active' : ''
            }`}
        >
          Params
        </button>
        <button
          onClick={() => setActiveTab('body')}
          className={`sub-tab-button py-3 px-4 text-sm font-medium ${activeTab === 'body' ? 'active' : ''
            }`}
        >
          Body
        </button>
        <button
          onClick={() => setActiveTab('headers')}
          className={`sub-tab-button py-3 px-4 text-sm font-medium ${activeTab === 'headers' ? 'active' : ''
            }`}
        >
          Headers
        </button>
        <button
          onClick={() => setActiveTab('suggestions')}
          className={`sub-tab-button py-3 px-4 text-sm font-medium ${activeTab === 'suggestions' ? 'active' : ''
            }`}
        >
          Suggestions
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-4">
        <div
          id="request-params-content"
          className={`${activeTab === 'params' ? '' : 'hidden'}`}
        >
          <KeyValueEditor
            items={params}
            onChange={setParams}
            keyPlaceholder="Param Key"
            valuePlaceholder="Value"
          />
        </div>

        <div
          id="request-body-content"
          className={`${activeTab === 'body' ? '' : 'hidden'}`}
        >
          <div className="editor-container w-full h-64 bg-gray-50 border border-gray-300 rounded-md overflow-auto">
            <Editor
              value={body}
              onValueChange={(code) => setBody(code)}
              highlight={(code) => highlight(code, languages.json, 'json')}
              padding={12}
              className="editor"
              style={{
                fontFamily: '"Fira code", "Fira Mono", monospace',
                fontSize: 14,
                lineHeight: 1.5,
              }}
              disabled={method === 'GET' || method === 'DELETE'}
            />
          </div>
        </div>

        <div
          id="request-headers-content"
          className={`${activeTab === 'headers' ? '' : 'hidden'}`}
        >
          <KeyValueEditor
            items={headers}
            onChange={setHeaders}
            keyPlaceholder="Header Name"
            valuePlaceholder="Header Value"
          />
        </div>

        <div
          id="request-suggestions-content"
          className={`${activeTab === 'suggestions' ? '' : 'hidden'}`}
        >
          <p className="text-sm text-gray-600 mb-4">
            Use these helpers to auto-fill the request bar (requires a selected
            collection).
          </p>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="p-4 bg-gray-50 rounded-lg border border-gray-200">
              <h3 className="font-semibold text-gray-800 mb-3">
                ID Operations
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
                Batch Operations
              </h3>
              <p className="text-xs text-gray-500 mb-3">
                Use the `_search` or `_insertMany` APIs for the selected
                collection.
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