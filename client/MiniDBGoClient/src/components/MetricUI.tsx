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

// Hằng số backend
const L0_COMPACTION_TRIGGER = 4
const L1_COMPACTION_TRIGGER_BYTES = 100 * 1024 * 1024 // 100MB

export function LevelBar({
  label,
  fileCount,
  totalSize,
  currentValue,
  maxValue,
  tooltip,
}: LevelBarProps) {
  // Tính toán phần trăm, đảm bảo không chia cho 0
  const percent =
    maxValue > 0 ? Math.min((currentValue / maxValue) * 100, 100) : 0

  // Quyết định màu sắc thanh tiến trình (theme sáng)
  let barColor = 'bg-blue-600' // Mặc định
  if (percent > 85) {
    barColor = 'bg-red-600' // Gần đầy
  } else if (percent > 60) {
    barColor = 'bg-yellow-500' // Hơn một nửa
  }

  return (
    <div className="mb-4" title={tooltip}>
      {/* Hàng 1: Tên và các chỉ số */}
      <div className="flex justify-between items-end mb-1 text-sm">
        <span className="font-semibold text-gray-700">{label}</span>
        <span className="text-gray-500">
          {fileCount} files / {formatBytes(totalSize)}
        </span>
      </div>

      {/* Hàng 2: Thanh tiến trình (chỉ hiển thị nếu có maxValue) */}
      {maxValue > 0 && (
        <div className="w-full bg-gray-200 rounded-full h-2.5">
          <div
            className={`h-2.5 rounded-full transition-all duration-300 ${barColor}`}
            style={{ width: `${percent}%` }}
          ></div>
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
      <h3 className="text-base font-semibold text-gray-700 mb-4">
        LSM Tree State
      </h3>
      <LevelBar
        label="Level 0 (L0)"
        fileCount={l0files}
        totalSize={l0bytes}
        currentValue={l0files}
        maxValue={L0_COMPACTION_TRIGGER}
        tooltip={`L0 sẽ nén xuống L1 khi đạt ${L0_COMPACTION_TRIGGER} files.`}
      />
      <LevelBar
        label="Level 1 (L1)"
        fileCount={l1files}
        totalSize={l1bytes}
        currentValue={l1bytes}
        maxValue={L1_COMPACTION_TRIGGER_BYTES}
        tooltip={`L1 sẽ nén xuống L2 khi vượt quá ${formatBytes(
          L1_COMPACTION_TRIGGER_BYTES,
        )}.`}
      />
      <LevelBar
        label="Level 2 (L2)"
        fileCount={l2files}
        totalSize={l2bytes}
        currentValue={0} // Không có thanh tiến trình
        maxValue={0}
      />
    </div>
  )
}