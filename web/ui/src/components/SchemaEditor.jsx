import { useState, useRef, useCallback, useEffect } from 'react'
import { getFieldConfig } from '../fieldTypeConfig.jsx'

const NODE_WIDTH = 220
const NODE_HEADER_H = 44
const NODE_ROW_H = 30
const GRID_COL_GAP = 60
const GRID_ROW_GAP = 40

/** Compute initial grid positions so nodes don't overlap */
function initialPositions(tables, storageKey) {
  const saved = (() => {
    try { return JSON.parse(localStorage.getItem(storageKey)) || {} } catch { return {} }
  })()

  const cols = Math.max(1, Math.floor((window.innerWidth - 100) / (NODE_WIDTH + GRID_COL_GAP)))
  const positions = {}
  tables.forEach((t, i) => {
    if (saved[t.id]) {
      positions[t.id] = saved[t.id]
    } else {
      const col = i % cols
      const row = Math.floor(i / cols)
      positions[t.id] = {
        x: 60 + col * (NODE_WIDTH + GRID_COL_GAP),
        y: 60 + row * (200 + GRID_ROW_GAP),
      }
    }
  })
  return positions
}

/** Build edges from relation columns */
function buildEdges(tables) {
  const edges = []
  const tableBySlug = Object.fromEntries(tables.map(t => [t.slug, t]))

  tables.forEach(srcTable => {
    ;(srcTable.columns || []).forEach((col, colIdx) => {
      if (col.field_type !== 'relation') return
      const meta = typeof col.meta === 'string' ? JSON.parse(col.meta || '{}') : (col.meta || {})
      const targetSlug = meta.target_table
      if (!targetSlug || !tableBySlug[targetSlug]) return
      edges.push({
        id: `${srcTable.id}-${col.id}`,
        srcTableId: srcTable.id,
        srcColIdx: colIdx,
        dstTableId: tableBySlug[targetSlug].id,
      })
    })
  })
  return edges
}

/** Compute anchor points on a node box */
function anchorPoint(pos, colIdx) {
  // source: right side, at the row of the relation column
  const y = pos.y + NODE_HEADER_H + colIdx * NODE_ROW_H + NODE_ROW_H / 2
  return { x: pos.x + NODE_WIDTH, y }
}

function dstAnchorPoint(pos) {
  // destination: left side, vertically centered
  return { x: pos.x, y: pos.y + NODE_HEADER_H / 2 }
}

/** SVG bezier path between two points */
function bezierPath(sx, sy, ex, ey) {
  const cp = Math.abs(ex - sx) * 0.5
  return `M ${sx} ${sy} C ${sx + cp} ${sy}, ${ex - cp} ${ey}, ${ex} ${ey}`
}

export function SchemaEditor({ tables, workspaceId, onEditTable }) {
  const storageKey = `erd-positions-${workspaceId}`
  const [positions, setPositions] = useState(() => initialPositions(tables, storageKey))
  const [dragging, setDragging] = useState(null) // { tableId, offsetX, offsetY }
  const svgRef = useRef(null)

  // Persist positions
  useEffect(() => {
    localStorage.setItem(storageKey, JSON.stringify(positions))
  }, [positions, storageKey])

  // Recalculate positions when tables change (new table added)
  useEffect(() => {
    setPositions(prev => {
      const next = { ...prev }
      const cols = Math.max(1, Math.floor((window.innerWidth - 100) / (NODE_WIDTH + GRID_COL_GAP)))
      const newTables = tables.filter(t => !next[t.id])
      newTables.forEach((t, i) => {
        const idx = tables.indexOf(t)
        const col = idx % cols
        const row = Math.floor(idx / cols)
        next[t.id] = { x: 60 + col * (NODE_WIDTH + GRID_COL_GAP), y: 60 + row * (200 + GRID_ROW_GAP) }
      })
      return next
    })
  }, [tables])

  const edges = buildEdges(tables)

  const onMouseDown = useCallback((e, tableId) => {
    e.stopPropagation()
    const pos = positions[tableId] || { x: 0, y: 0 }
    const svgRect = svgRef.current.getBoundingClientRect()
    setDragging({
      tableId,
      offsetX: e.clientX - svgRect.left - pos.x,
      offsetY: e.clientY - svgRect.top - pos.y,
    })
  }, [positions])

  const onMouseMove = useCallback((e) => {
    if (!dragging) return
    const svgRect = svgRef.current.getBoundingClientRect()
    const x = Math.max(0, e.clientX - svgRect.left - dragging.offsetX)
    const y = Math.max(0, e.clientY - svgRect.top - dragging.offsetY)
    setPositions(prev => ({ ...prev, [dragging.tableId]: { x, y } }))
  }, [dragging])

  const onMouseUp = useCallback(() => setDragging(null), [])

  // Touch support
  const onTouchStart = useCallback((e, tableId) => {
    const touch = e.touches[0]
    const pos = positions[tableId] || { x: 0, y: 0 }
    const svgRect = svgRef.current.getBoundingClientRect()
    setDragging({ tableId, offsetX: touch.clientX - svgRect.left - pos.x, offsetY: touch.clientY - svgRect.top - pos.y })
  }, [positions])

  const onTouchMove = useCallback((e) => {
    if (!dragging) return
    e.preventDefault()
    const touch = e.touches[0]
    const svgRect = svgRef.current.getBoundingClientRect()
    const x = Math.max(0, touch.clientX - svgRect.left - dragging.offsetX)
    const y = Math.max(0, touch.clientY - svgRect.top - dragging.offsetY)
    setPositions(prev => ({ ...prev, [dragging.tableId]: { x, y } }))
  }, [dragging])

  // Compute SVG canvas size
  const maxX = Math.max(800, ...tables.map(t => (positions[t.id]?.x || 0) + NODE_WIDTH + 60))
  const maxY = Math.max(500, ...tables.map(t => {
    const h = NODE_HEADER_H + (t.columns?.length || 0) * NODE_ROW_H + 20
    return (positions[t.id]?.y || 0) + h + 60
  }))

  return (
    <div className="erd-canvas-wrap">
      <svg
        ref={svgRef}
        width={maxX}
        height={maxY}
        className="erd-svg"
        onMouseMove={onMouseMove}
        onMouseUp={onMouseUp}
        onMouseLeave={onMouseUp}
        onTouchMove={onTouchMove}
        onTouchEnd={onMouseUp}
        style={{ cursor: dragging ? 'grabbing' : 'default' }}
      >
        <defs>
          <marker id="arrow" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
            <path d="M0,0 L0,6 L8,3 z" fill="var(--primary)" />
          </marker>
          {/* Grid background */}
          <pattern id="erd-grid" width="24" height="24" patternUnits="userSpaceOnUse">
            <circle cx="1" cy="1" r="1" fill="var(--border-light)" />
          </pattern>
        </defs>

        {/* Dot grid background */}
        <rect width={maxX} height={maxY} fill="url(#erd-grid)" />

        {/* Relation edges */}
        {edges.map(edge => {
          const srcPos = positions[edge.srcTableId]
          const dstPos = positions[edge.dstTableId]
          if (!srcPos || !dstPos) return null
          const src = anchorPoint(srcPos, edge.srcColIdx)
          const dst = dstAnchorPoint(dstPos)
          return (
            <path
              key={edge.id}
              d={bezierPath(src.x, src.y, dst.x, dst.y)}
              fill="none"
              stroke="var(--primary)"
              strokeWidth="2"
              strokeDasharray="6 3"
              markerEnd="url(#arrow)"
              opacity="0.8"
            />
          )
        })}

        {/* Table nodes */}
        {tables.map(table => {
          const pos = positions[table.id] || { x: 60, y: 60 }
          const cols = table.columns || []
          const nodeH = NODE_HEADER_H + cols.length * NODE_ROW_H + 8

          return (
            <g
              key={table.id}
              transform={`translate(${pos.x}, ${pos.y})`}
              style={{ cursor: dragging?.tableId === table.id ? 'grabbing' : 'grab' }}
              onMouseDown={(e) => onMouseDown(e, table.id)}
              onTouchStart={(e) => onTouchStart(e, table.id)}
            >
              {/* Drop shadow */}
              <rect
                x="4" y="4"
                width={NODE_WIDTH} height={nodeH}
                rx="10" ry="10"
                fill="var(--border-color)" opacity="0.25"
              />

              {/* Node body */}
              <rect
                width={NODE_WIDTH} height={nodeH}
                rx="10" ry="10"
                fill="var(--bg-elevated)"
                stroke="var(--border-color)"
                strokeWidth="2.5"
              />

              {/* Header */}
              <rect
                width={NODE_WIDTH} height={NODE_HEADER_H}
                rx="10" ry="10"
                fill="var(--primary-light)"
              />
              <rect
                y={NODE_HEADER_H - 10} width={NODE_WIDTH} height="10"
                fill="var(--primary-light)"
              />
              <rect
                y={NODE_HEADER_H - 1} width={NODE_WIDTH} height="2.5"
                fill="var(--border-color)"
              />

              {/* Table icon + name */}
              <text x="14" y="17" fontSize="11" fill="var(--primary)" fontWeight="700" fontFamily="monospace">⊞</text>
              <text
                x="30" y="18"
                fontSize="12.5" fontWeight="800"
                fill="var(--text)"
                fontFamily="Inter, system-ui, sans-serif"
              >
                {table.name.length > 18 ? table.name.slice(0, 17) + '…' : table.name}
              </text>
              <text x="30" y="34" fontSize="10" fill="var(--text-muted)" fontFamily="monospace">@{table.slug}</text>

              {/* Edit button */}
              <g
                style={{ cursor: 'pointer' }}
                onClick={(e) => { e.stopPropagation(); onEditTable(table) }}
              >
                <rect x={NODE_WIDTH - 30} y="8" width="22" height="22" rx="6" fill="var(--bg-elevated)" stroke="var(--border-light)" strokeWidth="1.5" />
                <text x={NODE_WIDTH - 22} y="23" fontSize="11" fill="var(--text-secondary)" textAnchor="middle">✎</text>
              </g>

              {/* Columns */}
              {cols.map((col, i) => {
                const cfg = getFieldConfig(col.field_type)
                const y = NODE_HEADER_H + i * NODE_ROW_H
                const isRelation = col.field_type === 'relation'
                return (
                  <g key={col.id}>
                    {/* Alternating row bg */}
                    {i % 2 === 1 && (
                      <rect x="0" y={y} width={NODE_WIDTH} height={NODE_ROW_H} fill="var(--bg-surface)" />
                    )}
                    {/* Field type dot */}
                    <circle cx="16" cy={y + NODE_ROW_H / 2} r="5" fill={cfg.color} />
                    {/* Column name */}
                    <text
                      x="28" y={y + NODE_ROW_H / 2 + 4}
                      fontSize="11.5" fill={isRelation ? 'var(--primary)' : 'var(--text)'}
                      fontFamily="Inter, system-ui, sans-serif"
                      fontWeight={isRelation ? '700' : '400'}
                    >
                      {col.name.length > 16 ? col.name.slice(0, 15) + '…' : col.name}
                    </text>
                    {/* Field type label */}
                    <text
                      x={NODE_WIDTH - 8} y={y + NODE_ROW_H / 2 + 4}
                      fontSize="9" fill="var(--text-muted)"
                      fontFamily="monospace" textAnchor="end"
                    >
                      {col.field_type}
                    </text>
                    {/* Relation anchor mark */}
                    {isRelation && (
                      <circle cx={NODE_WIDTH} cy={y + NODE_ROW_H / 2} r="4" fill="var(--primary)" />
                    )}
                  </g>
                )
              })}

              {/* Bottom border on last row */}
              <rect y={nodeH - 1} width={NODE_WIDTH} height="1" fill="var(--border-light)" />
            </g>
          )
        })}
      </svg>
    </div>
  )
}
