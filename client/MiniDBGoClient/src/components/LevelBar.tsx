interface LevelBarProps {
  label: string
  fileCount: number
  totalSize: string
  currentValue: number
  maxValue: number
  tooltip?: string
}

export default function LevelBar({
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

  // Quyết định màu sắc thanh tiến trình
  let barColor = 'bg-green-500' // Mặc định
  if (percent > 85) {
    barColor = 'bg-red-500' // Gần đầy
  } else if (percent > 60) {
    barColor = 'bg-yellow-500' // Hơn một nửa
  }

  return (
    <div className="mb-4" title={tooltip}>
      {/* Hàng 1: Tên và các chỉ số */}
      <div className="flex justify-between items-end mb-1 text-sm">
        <span className="font-semibold text-gray-300">{label}</span>
        <span className="text-gray-400">
          {fileCount} files / {totalSize}
        </span>
      </div>

      {/* Hàng 2: Thanh tiến trình (chỉ hiển thị nếu có maxValue) */}
      {maxValue > 0 && (
        <div className="w-full bg-gray-700 rounded-full h-2.5">
          <div
            className={`h-2.5 rounded-full transition-all duration-300 ${barColor}`}
            style={{ width: `${percent}%` }}
          ></div>
        </div>
      )}
    </div>
  )
}