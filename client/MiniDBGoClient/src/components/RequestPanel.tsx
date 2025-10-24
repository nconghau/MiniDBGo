import React, { useEffect, useState } from 'react';
import { Send, Plus, Trash2, Combine, Eraser, ChevronDown } from 'lucide-react';
import Editor from 'react-simple-code-editor';
import { highlight, languages } from 'prismjs';
import 'prismjs/components/prism-json';
import 'prismjs/themes/prism-tomorrow.css';
import { KeyValueItem } from '../data/api';

// --- Component UI con cho Key-Value (Đã sửa) ---
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
            className="form-checkbox h-5 w-5 text-primary-600 rounded focus:ring-primary-500"
            checked={item.enabled}
            onChange={(e) => handleUpdate(item.id, 'enabled', e.target.checked)}
          />
          <input
            type="text"
            // --- CẬP NHẬT: Dùng border slate, focus-within primary ---
            className="flex-1 min-w-0 px-3 py-2 text-sm font-mono bg-white border border-slate-300 rounded-md focus:outline-none focus:ring-0 focus:border-primary-500"
            placeholder={keyPlaceholder}
            value={item.key}
            onChange={(e) => handleUpdate(item.id, 'key', e.target.value)}
          />
          <input
            type="text"
            // --- CẬP NHẬT: Dùng border slate, focus-within primary ---
            className="flex-1 min-w-0 px-3 py-2 text-sm font-mono bg-white border border-slate-300 rounded-md focus:outline-none focus:ring-0 focus:border-primary-500"
            placeholder={valuePlaceholder}
            value={item.value}
            onChange={(e) => handleUpdate(item.id, 'value', e.target.value)}
          />
          <button
            onClick={() => handleRemove(item.id)}
            className="p-2 text-gray-500 hover:text-red-600 focus:outline-none focus:ring-1 focus:ring-red-500 rounded"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      ))}
      <button
        onClick={handleAdd}
        className="flex items-center gap-2 px-3 py-2 text-sm font-medium text-primary-600 hover:bg-primary-50 rounded-md focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-primary-500"
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
const DEFAULT_BODY_EMPTY = '{\n  \n}';

// --- CẬP NHẬT: Mockup data cho "Smart Suggestions" ---
// (Logic thực tế sẽ cần phân tích các collection)
const MOCK_FIELD_SUGGESTIONS = [
  "_id", "name", "category", "price", "email", "address", "created_at"
];

export default function RequestPanel({
  activeCollection,
  loading,
  onSend,
}: RequestPanelProps) {
  const [method, setMethod] = useState<Method>('POST');
  const [path, setPath] = useState('/{collection}/_search');
  const [body, setBody] = useState(DEFAULT_BODY_SEARCH);
  const [helperId, setHelperId] = useState('p1');
  type RequestTab = 'params_headers' | 'body';
  const [activeTab, setActiveTab] = useState<RequestTab>('body');
  const [params, setParams] = useState<KeyValueItem[]>([]);
  const [headers, setHeaders] = useState<KeyValueItem[]>([]);

  const [jsonError, setJsonError] = useState<string | null>(null);

  useEffect(() => {
    if (!activeCollection) {
      setPath('/{collection}/_search');
    } else {
      if (path.startsWith('/{collection}')) {
        const newPath = path.replace('/{collection}', `/${activeCollection}`);
        setPath(newPath);
      }
    }
  }, [activeCollection]);

  const getPath = (id: string) =>
    `/${activeCollection || '{collection}'}/${id}`;

  const handleHelperGet = () => {
    setMethod('GET');
    setPath(getPath(helperId));
    setBody('');
    setActiveTab('body');
    setJsonError(null);
  };
  const handleHelperPut = () => {
    setMethod('PUT');
    setPath(getPath(helperId));
    setBody(DEFAULT_BODY_PUT);
    setActiveTab('body');
    setJsonError(null);
  };
  const handleHelperDelete = () => {
    setMethod('DELETE');
    setPath(getPath(helperId));
    setBody('');
    setActiveTab('body');
    setJsonError(null);
  };
  const handleHelperSearch = () => {
    setMethod('POST');
    setPath(`/${activeCollection || '{collection}'}/_search`);
    setBody(DEFAULT_BODY_SEARCH);
    setActiveTab('body');
    setJsonError(null);
  };
  const handleHelperInsertMany = () => {
    setMethod('POST');
    setPath(`/${activeCollection || '{collection}'}/_insertMany`);
    setBody(DEFAULT_BODY_INSERT_MANY);
    setActiveTab('body');
    setJsonError(null);
  };

  const handleSend = () => {
    if (jsonError && (method === 'POST' || method === 'PUT')) {
      alert(`Invalid JSON: ${jsonError}`);
      return;
    }
    const bodyToSend = (method === 'POST' || method === 'PUT') ? body : null;
    onSend(method, path, bodyToSend, params, headers);
  };

  const handleFormatJson = () => {
    try {
      if (body.trim() === '' || body.trim() === '{}') {
        setBody(DEFAULT_BODY_EMPTY);
        setJsonError(null);
        return;
      }
      const parsed = JSON.parse(body);
      const formatted = JSON.stringify(parsed, null, 2);
      setBody(formatted);
      setJsonError(null);
    } catch (e: any) {
      setJsonError(`Invalid JSON, cannot format: ${e.message}`);
    }
  };

  const handleClearJson = () => {
    setBody(DEFAULT_BODY_EMPTY);
    setJsonError(null);
  };

  const handleBodyChange = (code: string) => {
    setBody(code);
    try {
      if (code.trim() === '' || method === 'GET' || method === 'DELETE') {
        setJsonError(null);
        return;
      }
      JSON.parse(code);
      setJsonError(null); // Valid JSON
    } catch (e: any) {
      setJsonError(e.message); // Invalid JSON
    }
  };


  const isSendDisabled = loading || path.includes('{collection}');

  return (
    // --- CẬP NHẬT: Bỏ card, thêm padding ---
    <div
      id="request-panel"
      className="flex-1 overflow-hidden flex flex-col p-4"
    >
      {/* --- CẬP NHẬT LỚN: Thanh Request Bar giống mẫu --- */}
      <div className="flex-shrink-0 flex items-center bg-slate-100 rounded-lg border border-slate-200 focus-within:border-primary-500 focus-within:ring-1 focus-within:ring-primary-500">
        <div className="relative">
          <select
            id="method-toggle-button"
            data-method={method}
            value={method}
            onChange={(e) => setMethod(e.target.value as Method)}
            className="appearance-none h-full pl-4 pr-10 py-2.5 text-sm font-semibold text-white bg-primary-600 rounded-l-lg focus:outline-none"
          >
            <option value="POST">POST</option>
            <option value="GET">GET</option>
            <option value="PUT">PUT</option>
            <option value="DELETE">DELETE</option>
          </select>
          <ChevronDown className="w-4 h-4 text-white absolute right-3 top-1/2 -translate-y-1/2 pointer-events-none" />
        </div>
        <input
          value={path}
          onChange={(e) => setPath(e.target.value)}
          type="text"
          id="rest-path"
          className="flex-1 min-w-0 px-4 py-2.5 text-sm font-mono bg-transparent border-l border-slate-200 focus:outline-none focus:ring-0"
          placeholder="/{collection}/{id}"
        />
        <button
          id="btn-send-rest"
          onClick={handleSend}
          disabled={isSendDisabled}
          className="flex-shrink-0 flex items-center justify-center gap-2 px-5 py-2.5 text-sm font-medium text-white bg-primary-600 rounded-r-lg hover:bg-primary-700 focus:outline-none disabled:opacity-50"
        >
          <Send className="w-4 h-4" />
          Send
        </button>
      </div>

      {/* --- CẬP NHẬT: Thanh Tabs (thêm mt-4) --- */}
      <div
        id="request-sub-tabs"
        className="flex-shrink-0 flex border-b border-gray-200 px-1 mt-4"
      >
        <button
          onClick={() => setActiveTab('params_headers')}
          className={`sub-tab-button py-3 px-4 text-sm font-medium focus:outline-none ${activeTab === 'params_headers' ? 'active' : 'text-slate-600 hover:text-slate-800'
            }`}
        >
          Params / Headers
        </button>
        <button
          onClick={() => setActiveTab('body')}
          className={`sub-tab-button py-3 px-4 text-sm font-medium focus:outline-none ${activeTab === 'body' ? 'active' : 'text-slate-600 hover:text-slate-800'
            }`}
        >
          Body / Suggestions
        </button>
      </div>

      {/* --- CẬP NHẬT: Nội dung (thêm pt-4) --- */}
      <div className="flex-1 overflow-y-auto pt-4 space-y-6">
        {/* --- NỘI DUNG TAB GỘP: PARAMS & HEADERS --- */}
        <div
          id="request-params-headers-content"
          className={`${activeTab === 'params_headers' ? '' : 'hidden'} space-y-6`}
        >
          <div>
            <h3 className="text-sm font-semibold text-gray-700 mb-2">Query Params</h3>
            <KeyValueEditor
              items={params}
              onChange={setParams}
              keyPlaceholder="Param Key"
              valuePlaceholder="Value"
            />
          </div>
          <div>
            <h3 className="text-sm font-semibold text-gray-700 mb-2">Headers</h3>
            <KeyValueEditor
              items={headers}
              onChange={setHeaders}
              keyPlaceholder="Header Name"
              valuePlaceholder="Header Value"
            />
          </div>
        </div>

        {/* --- NỘI DUNG TAB GỘP: BODY & SUGGESTIONS --- */}
        <div
          id="request-body-suggestions-content"
          className={`${activeTab === 'body' ? 'flex flex-col gap-6' : 'hidden'}`}
        >
          {/* --- CẬP NHẬT: Editor JSON --- */}
          <div>
            <div className="flex justify-between items-center mb-2">
              <h3 className="text-sm font-semibold text-gray-700">Request Body (JSON)</h3>
              <div className="flex items-center gap-2">
                <button
                  onClick={handleFormatJson}
                  className="flex items-center gap-1.5 px-3 py-1 text-xs font-medium text-primary-600 hover:bg-primary-50 rounded-md focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-primary-500"
                >
                  <Combine className="w-3 h-3" />
                  Format
                </button>
                <button
                  onClick={handleClearJson}
                  className="flex items-center gap-1.5 px-3 py-1 text-xs font-medium text-red-600 hover:bg-red-50 rounded-md focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-red-500"
                >
                  <Eraser className="w-3 h-3" />
                  Clear
                </button>
              </div>
            </div>

            {/* --- CẬP NHẬT: Logic border (dùng slate) --- */}
            <div className={`editor-container w-full h-64 bg-slate-50 rounded-md overflow-hidden transition-colors ${jsonError
                ? 'border-2 border-red-500' // Lỗi: border đỏ dày
                : 'border border-slate-300 focus-within:border-primary-500 focus-within:border-2' // Mặc định: border xám, Focus: border tím
              }`}>
              <Editor
                value={body}
                onValueChange={handleBodyChange}
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
            {jsonError && (
              <p className="mt-2 text-xs text-red-600 font-mono">
                {jsonError}
              </p>
            )}
          </div>

          {/* --- CẬP NHẬT: Suggestions --- */}
          <div>
            <h3 className="text-sm font-semibold text-gray-700 mb-2">Suggestions</h3>

            {/* --- MỚI: "Smart Suggestions" Mockup --- */}
            <div className="mb-4">
              <h4 className="text-xs font-semibold text-gray-600 mb-2">Field Suggestions (Click to copy)</h4>
              <div className="flex flex-wrap gap-2">
                {MOCK_FIELD_SUGGESTIONS.map((field) => (
                  <button
                    key={field}
                    title={`Copy "${field}"`}
                    onClick={() => navigator.clipboard.writeText(`"${field}"`)}
                    className="bg-slate-200 text-slate-700 font-mono text-xs px-2.5 py-1 rounded-full cursor-pointer hover:bg-slate-300 transition-colors"
                  >
                    {field}
                  </button>
                ))}
              </div>
            </div>

            <p className="text-xs text-gray-600 mb-4">
              Hoặc dùng các nút điền nhanh (cần chọn collection).
            </p>
            <div className="mb-3">
              <label htmlFor="helper-id" className="block text-xs font-medium text-gray-600 mb-1">
                Document ID
              </label>
              <input
                id="helper-id"
                value={helperId}
                onChange={(e) => setHelperId(e.target.value)}
                className="w-full max-w-xs px-3 py-2 text-sm font-mono bg-white border border-slate-300 rounded-md focus:outline-none focus:ring-0 focus:border-primary-500"
              />
            </div>
            <div className="grid grid-cols-3 md:grid-cols-5 gap-2">
              <button
                onClick={handleHelperGet}
                className="px-3 py-2 text-sm font-medium text-green-700 bg-green-100 rounded-md hover:bg-green-200 focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-green-500"
              > GET </button>
              <button
                onClick={handleHelperPut}
                className="px-3 py-2 text-sm font-medium text-yellow-700 bg-yellow-100 rounded-md hover:bg-yellow-200 focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-yellow-500"
              > PUT </button>
              <button
                onClick={handleHelperDelete}
                className="px-3 py-2 text-sm font-medium text-red-700 bg-red-100 rounded-md hover:bg-red-200 focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-red-500"
              > DELETE </button>
              <button
                onClick={handleHelperSearch}
                className="px-3 py-2 text-sm font-medium text-blue-700 bg-blue-100 rounded-md hover:bg-blue-200 focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-blue-500"
              > _search </button>
              <button
                onClick={handleHelperInsertMany}
                className="px-3 py-2 text-sm font-medium text-purple-700 bg-purple-100 rounded-md hover:bg-purple-200 focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-purple-500"
              > _insertMany </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}