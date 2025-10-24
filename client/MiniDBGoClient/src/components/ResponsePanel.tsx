import React, { useState, useEffect, useRef } from 'react';
import hljs from 'highlight.js/lib/core';
import json from 'highlight.js/lib/languages/json';
import { ResponseData } from '../data/api';

// Đăng ký language
hljs.registerLanguage('json', json);

interface ResponsePanelProps {
  response: ResponseData | null;
  loading: boolean;
}

export default function ResponsePanel({
  response,
  loading,
}: ResponsePanelProps) {
  // Xóa local state [cite: 31-33]
  const [activeTab, setActiveTab] = useState('response-body-content');
  const codeRef = useRef<HTMLElement>(null);
  const headersRef = useRef<HTMLElement>(null);

  // Định dạng nội dung body
  let bodyText = '// Phản hồi API sẽ xuất hiện ở đây';
  if (loading) {
    bodyText = '// Đang tải...';
  } else if (response) {
    if (response.isError && response.error) {
      bodyText = response.error;
    } else if (typeof response.body === 'object') {
      bodyText = JSON.stringify(response.body, null, 2);
    } else {
      bodyText = String(response.body);
    }
  }

  // Định dạng nội dung headers
  let headersText = '// Headers phản hồi sẽ xuất hiện ở đây';
  if (loading) {
    headersText = '// Đang tải...';
  } else if (response && response.headers) {
    headersText = JSON.stringify(response.headers, null, 2);
  }

  // Chạy highlight.js khi bodyText thay đổi
  useEffect(() => {
    if (codeRef.current) {
      hljs.highlightElement(codeRef.current);
    }
  }, [bodyText]);

  // Chạy highlight.js khi headersText thay đổi và tab active
  useEffect(() => {
    if (headersRef.current && activeTab === 'response-headers-content') {
      hljs.highlightElement(headersRef.current);
    }
  }, [headersText, activeTab]);

  return (
    <div
      id="response-panel"
      className="flex-1 bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden flex flex-col"
    >
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
          <pre className="h-full">
            <code
              ref={codeRef}
              id="response-body-code"
              className={`language-json h-full ${response?.isError ? 'error' : ''
                }`}
            >
              {bodyText}
            </code>
          </pre>
        </div>
        {/* Tab Headers */}
        <div
          id="response-headers-content"
          className={`${activeTab === 'response-headers-content' ? '' : 'hidden'
            } h-full`}
        >
          <pre className="h-full">
            <code ref={headersRef} className="language-json h-full p-5">
              {headersText}
            </code>
          </pre>
        </div>
      </div>
    </div>
  );
}