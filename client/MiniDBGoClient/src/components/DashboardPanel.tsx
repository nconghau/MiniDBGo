import React, { useState, useEffect } from 'react'
import { fetchApi } from '../data/api'
import { Line } from 'react-chartjs-2'
import {
  Chart,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js'
import { Loader2, Activity, Cpu, MemoryStick, Zap } from 'lucide-react'

Chart.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler,
)

// === THÊM VÀO: Bộ định dạng số theo locale vi-VN ===
// (16384 -> "16.384")
const intFormatter = new Intl.NumberFormat('vi-VN', {
  minimumFractionDigits: 0,
  maximumFractionDigits: 0,
})
// (73.57 -> "73,57")
const floatFormatter = new Intl.NumberFormat('vi-VN', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
})
// === KẾT THÚC THÊM ===

interface StatsData {
  process_cpu_percent: number
  process_rss_mb: number
  process_rss_limit_mb: number
  go_num_goroutine: number
  system_cpu_percent: number
  go_alloc_mb: number
}

const MAX_DATA_POINTS = 30

export default function DashboardPanel() {
  const [stats, setStats] = useState<StatsData | null>(null)
  const [history, setHistory] = useState<StatsData[]>([])
  const [loading, setLoading] = useState(true)

  // Hàm lấy dữ liệu (Giữ nguyên)
  const fetchStats = async () => {
    try {
      const res = await fetchApi('GET', '/stats', null, [], [])
      if (!res.isError) {
        const data = res.body as StatsData
        setStats(data)
        setHistory((prev) => {
          const newHistory = [...prev, data]
          if (newHistory.length > MAX_DATA_POINTS) {
            return newHistory.slice(newHistory.length - MAX_DATA_POINTS)
          }
          return newHistory
        })
      }
    } catch (e) {
      console.error('Failed to fetch stats:', e)
    } finally {
      setLoading(false)
    }
  }

  // Thiết lập Polling (Giữ nguyên)
  useEffect(() => {
    fetchStats()
    const intervalId = setInterval(fetchStats, 2000)
    return () => clearInterval(intervalId)
  }, [])

  // --- Dữ liệu và Tùy chọn chung (Giữ nguyên) ---
  const labels = history.map((_, i) => i.toString())
  const baseChartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { display: false },
      tooltip: { mode: 'index' as const, intersect: false },
    },
    scales: {
      x: {
        ticks: { display: false },
        grid: { display: false },
      },
    },
    elements: { point: { radius: 0 } },
  }

  // --- Biểu đồ CPU (Giữ nguyên) ---
  const cpuChartData = {
    labels,
    datasets: [
      {
        label: 'Process CPU (%)',
        data: history.map((h) => h.process_cpu_percent),
        borderColor: '#4f46e5',
        backgroundColor: '#e0e7ff',
        fill: true,
        tension: 0.3,
      },
      {
        label: 'System CPU (%)',
        data: history.map((h) => h.system_cpu_percent),
        borderColor: '#6b7280',
        backgroundColor: '#e5e7eb',
        fill: true,
        tension: 0.3,
      },
    ],
  }
  const cpuChartOptions = {
    ...baseChartOptions,
    plugins: {
      ...baseChartOptions.plugins,
      legend: { display: true, position: 'bottom' as const },
    },
    scales: {
      ...baseChartOptions.scales,
      y: {
        title: { display: true, text: 'CPU (%)' },
        min: 0,
        max: 100,
      },
    },
  }

  // --- Biểu đồ RAM (Giữ nguyên) ---
  const ramChartData = {
    labels,
    datasets: [
      {
        label: 'Process RAM (MB)',
        data: history.map((h) => h.process_rss_mb),
        borderColor: '#059669',
        backgroundColor: '#a7f3d0',
        fill: true,
        tension: 0.3,
      },
    ],
  }
  const ramChartOptions = {
    ...baseChartOptions,
    scales: {
      ...baseChartOptions.scales,
      y: {
        title: { display: true, text: 'RAM (MB)' },
        min: 0,
        max:
          stats && stats.process_rss_limit_mb > 0
            ? stats.process_rss_limit_mb
            : undefined,
      },
    },
  }

  // --- Biểu đồ Goroutines (Giữ nguyên) ---
  const goroutineChartData = {
    labels,
    datasets: [
      {
        label: 'Goroutines',
        data: history.map((h) => h.go_num_goroutine),
        borderColor: '#f59e0b',
        backgroundColor: '#fde68a',
        fill: true,
        tension: 0.3,
      },
    ],
  }
  const goroutineChartOptions = {
    ...baseChartOptions,
    scales: {
      ...baseChartOptions.scales,
      y: {
        title: { display: true, text: 'Count' },
        min: 0,
        ticks: { stepSize: 1 },
      },
    },
  }

  return (
    <div className="flex-1 overflow-y-auto p-4 md:p-6">
      <h2 className="text-xl font-semibold text-gray-800 mb-4">Dashboard</h2>

      {/* === CẬP NHẬT: Sử dụng formatter cho các giá trị StatCard === */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <StatCard
          icon={Cpu}
          label="Process CPU"
          value={
            stats
              ? `${floatFormatter.format(stats.process_cpu_percent)} %`
              : '...'
          }
          loading={loading}
        />
        <StatCard
          icon={Zap}
          label="System CPU"
          value={
            stats
              ? `${floatFormatter.format(stats.system_cpu_percent)} %`
              : '...'
          }
          loading={loading}
        />

        <StatCard
          icon={MemoryStick}
          label="Process RAM (RSS)"
          value={
            loading || !stats
              ? '...'
              : stats.process_rss_limit_mb > 0
                ? `${floatFormatter.format(
                  stats.process_rss_mb,
                )} / ${intFormatter.format(stats.process_rss_limit_mb)} MB`
                : `${floatFormatter.format(stats.process_rss_mb)} MB`
          }
          loading={loading}
        />

        <StatCard
          icon={Activity}
          label="Goroutines"
          value={
            stats ? intFormatter.format(stats.go_num_goroutine) : '...'
          }
          loading={loading}
        />

      </div>

      {/* Phần biểu đồ (GiÃ nguyên) */}
      <div className="space-y-4">
        <div className="h-64 bg-white p-4 rounded-lg shadow-sm border border-gray-200">
          <h3 className="text-base font-semibold text-gray-700 mb-2">
            CPU Usage
          </h3>
          <div className="h-48">
            {loading && history.length === 0 ? (
              <LoadingSpinner />
            ) : (
              <Line options={cpuChartOptions} data={cpuChartData} />
            )}
          </div>
        </div>

        <div className="h-64 bg-white p-4 rounded-lg shadow-sm border border-gray-200">
          <h3 className="text-base font-semibold text-gray-700 mb-2">
            Process RAM (RSS)
          </h3>
          <div className="h-48">
            {loading && history.length === 0 ? (
              <LoadingSpinner />
            ) : (
              <Line options={ramChartOptions} data={ramChartData} />
            )}
          </div>
        </div>

        <div className="h-64 bg-white p-4 rounded-lg shadow-sm border border-gray-200">
          <h3 className="text-base font-semibold text-gray-700 mb-2">
            Active Goroutines
          </h3>
          <div className="h-48">
            {loading && history.length === 0 ? (
              <LoadingSpinner />
            ) : (
              <Line options={goroutineChartOptions} data={goroutineChartData} />
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

// StatCard (Giữ nguyên)
interface StatCardProps {
  icon: React.ElementType
  label: string
  value: string
  loading: boolean
}
function StatCard({ icon: Icon, label, value, loading }: StatCardProps) {
  return (
    <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-200 flex items-center gap-4">
      <div className="flex-shrink-0 p-3 rounded-full bg-primary-100 text-primary-600">
        <Icon className="w-5 h-5" />
      </div>
      <div>
        <p className="text-sm font-medium text-gray-500">{label}</p>
        {loading && value === '...' ? (
          <div className="h-6 w-20 bg-gray-200 rounded animate-pulse mt-1" />
        ) : (
          <p className="text-2xl font-semibold text-gray-900">{value}</p>
        )}
      </div>
    </div>
  )
}

// LoadingSpinner (Giữ nguyên)
const LoadingSpinner = () => (
  <div className="flex items-center justify-center h-full">
    <Loader2 className="w-8 h-8 text-primary-600 animate-spin" />
  </div>
)