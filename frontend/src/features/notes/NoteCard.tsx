import { memo } from 'react'
import { Archive, Trash2, FileDown, Sparkles } from 'lucide-react'
import type { NoteItem } from '../../shared/lib/types.ts'

type Props = {
  note: NoteItem
  onOpen: (id: string) => void
  onArchive: (note: NoteItem) => void
  onExport: (id: string) => void
  onDelete: (note: NoteItem) => void
  onAskAi: (note: NoteItem) => void
}

const actionBtn: React.CSSProperties = {
  background: 'none',
  border: 'none',
  cursor: 'pointer',
  color: 'var(--text-secondary)',
  display: 'flex',
  alignItems: 'center',
  padding: 4,
}

function NoteCardBase({ note, onOpen, onArchive, onExport, onDelete, onAskAi }: Props) {
  return (
    <article
      className="note-card"
      role="button"
      tabIndex={0}
      onClick={() => onOpen(note.id)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          onOpen(note.id)
        }
      }}
      style={note.archived ? { opacity: 0.55 } : undefined}
    >
      <div className="note-title">{note.title}</div>
      <div className="note-preview">
        {note.body.slice(0, 120).replace(/[#*`>\n]/g, ' ')}
      </div>
      {(note.tags ?? []).length > 0 && (
        <div className="note-tags">
          {note.tags.map((t) => (
            <span key={t} className="note-tag">
              #{t}
            </span>
          ))}
        </div>
      )}
      <div className="note-meta">
        <span>{new Date(note.updatedAt).toLocaleDateString()}</span>
        <div style={{ display: 'flex', gap: 4 }}>
          <button
            style={actionBtn}
            aria-label="问 AI"
            title="问 AI"
            onClick={(e) => {
              e.stopPropagation()
              onAskAi(note)
            }}
          >
            <Sparkles size={13} />
          </button>
          <button
            style={actionBtn}
            aria-label={note.archived ? '取消归档' : '归档'}
            title={note.archived ? '取消归档' : '归档'}
            onClick={(e) => {
              e.stopPropagation()
              onArchive(note)
            }}
          >
            <Archive size={13} />
          </button>
          <button
            style={actionBtn}
            aria-label="导出 Markdown"
            title="导出 Markdown"
            onClick={(e) => {
              e.stopPropagation()
              onExport(note.id)
            }}
          >
            <FileDown size={13} />
          </button>
          <button
            style={actionBtn}
            aria-label="删除"
            title="删除"
            onClick={(e) => {
              e.stopPropagation()
              onDelete(note)
            }}
          >
            <Trash2 size={13} />
          </button>
        </div>
      </div>
    </article>
  )
}

// memo 避免列表重渲染时未变更的卡片重新渲染（回调需保持稳定引用）
export const NoteCard = memo(NoteCardBase)
