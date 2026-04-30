const colors = {
  high: 'bg-red-500/20 text-red-400 border-red-500/30',
  medium: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
  low: 'bg-green-500/20 text-green-400 border-green-500/30',
}

export default function RiskBadge({ level }) {
  const cls = colors[level] || colors.low
  return (
    <span className={`inline-flex min-w-16 items-center justify-center rounded border px-2 py-0.5 text-xs font-semibold uppercase ${cls}`}>
      {level}
    </span>
  )
}
