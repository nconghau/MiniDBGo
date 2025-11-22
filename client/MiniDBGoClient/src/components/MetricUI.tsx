import { formatBytes } from '../data/api'
import { EngineMetrics } from './DashboardPanel'

interface LevelBarProps {
  label: string
  fileCount: number
  totalSize: number // Truyền vào số bytes
  currentValue: number
  maxValue: number
  tooltip?: string
}

// Hằng số cấu hình (LSM Tree Configuration)
const L0_COMPACTION_TRIGGER = 4 // Số lượng file
const L1_COMPACTION_TRIGGER_BYTES = 100 * 1024 * 1024 // 100MB
// L2 thường có kích thước gấp 10 lần L1 trong kiến trúc LSM tiêu chuẩn
const L2_REFERENCE_CAPACITY_BYTES = 10 * L1_COMPACTION_TRIGGER_BYTES // 1GB

export function LevelBar({
  label,
  fileCount,
  totalSize,
  currentValue,
  maxValue,
  tooltip,
}: LevelBarProps) {
  // Tính toán phần trăm, đảm bảo không chia cho 0 và tối đa là 100%
  const rawPercent = maxValue > 0 ? (currentValue / maxValue) * 100 : 0
  const percent = Math.min(rawPercent, 100)

  // Quyết định màu sắc thanh tiến trình
  let barColor = 'bg-blue-600' // Mặc định (xanh dương)
  
  if (percent >= 100) {
    barColor = 'bg-purple-600' // Đầy/Vượt ngưỡng (Tím - biểu thị trạng thái bão hòa)
  } else if (percent > 85) {
    barColor = 'bg-orange-500' // Gần đầy (Cam)
  } else if (percent > 60) {
    barColor = 'bg-yellow-500' // Hơn một nửa (Vàng)
  }

  return (
    <div className="mb-4 group relative cursor-help">
      {/* Hàng 1: Tên và các chỉ số */}
      <div className="flex justify-between items-end mb-1 text-sm">
        <span className="font-semibold text-gray-700">{label}</span>
        <span className="text-gray-500 font-mono text-xs">
          {fileCount} files / {formatBytes(totalSize)}
        </span>
      </div>

      {/* Hàng 2: Thanh tiến trình */}
      <div className="w-full bg-gray-200 rounded-full h-2.5 overflow-hidden">
        <div
          className={`h-2.5 rounded-full transition-all duration-500 ease-out ${barColor}`}
          style={{ width: `${percent}%` }}
        ></div>
      </div>

      {/* Tooltip tùy chỉnh (hiện khi hover) */}
      {tooltip && (
        <div className="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 px-2 py-1 bg-gray-800 text-white text-xs rounded opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap pointer-events-none z-10">
          {tooltip}
          {/* Mũi tên tooltip */}
          <div className="absolute top-full left-1/2 transform -translate-x-1/2 border-4 border-transparent border-t-gray-800"></div>
        </div>
      )}
    </div>
  )
}

/**
 * 3. LsmTreeMetrics (Component cho trạng thái Đĩa/LSM)
 */
export function LsmTreeMetrics({ metrics }: { metrics: EngineMetrics | null }) {
  const l0files = metrics?.level_0_files ?? 0
  const l0bytes = metrics?.level_0_bytes ?? 0
  const l1files = metrics?.level_1_files ?? 0
  const l1bytes = metrics?.level_1_bytes ?? 0
  const l2files = metrics?.level_2_files ?? 0
  const l2bytes = metrics?.level_2_bytes ?? 0

  return (
    <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-200">
      <div className="flex justify-between items-center mb-4">
        <h3 className="text-base font-semibold text-gray-700">
          LSM Tree State
        </h3>
        <span className="text-xs text-gray-400 bg-gray-100 px-2 py-1 rounded">
          Total Size: {formatBytes(l0bytes + l1bytes + l2bytes)}
        </span>
      </div>

      {/* LEVEL 0 */}
      <LevelBar
        label="Level 0 (MemTable Flush)"
        fileCount={l0files}
        totalSize={l0bytes}
        currentValue={l0files}
        maxValue={L0_COMPACTION_TRIGGER}
        tooltip={`Kích hoạt nén xuống L1 khi đạt ${L0_COMPACTION_TRIGGER} files (Hiện tại: ${l0files})`}
      />

      {/* LEVEL 1 */}
      <LevelBar
        label="Level 1 (Compacted)"
        fileCount={l1files}
        totalSize={l1bytes}
        currentValue={l1bytes}
        maxValue={L1_COMPACTION_TRIGGER_BYTES}
        tooltip={`Kích hoạt nén xuống L2 khi vượt quá ${formatBytes(L1_COMPACTION_TRIGGER_BYTES)} (Hiện tại: ${((l1bytes / L1_COMPACTION_TRIGGER_BYTES) * 100).toFixed(1)}%)`}
      />

      {/* LEVEL 2 */}
      <LevelBar
        label="Level 2 (Archive/Cold Data)"
        fileCount={l2files}
        totalSize={l2bytes}
        currentValue={l2bytes}
        maxValue={L2_REFERENCE_CAPACITY_BYTES}
        tooltip={`Dữ liệu tầng sâu. Dung lượng tham chiếu hiển thị: ${formatBytes(L2_REFERENCE_CAPACITY_BYTES)} (Hệ số x10 so với L1)`}
      />
    </div>
  )
}