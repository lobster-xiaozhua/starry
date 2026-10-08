import { useEffect, useState, useCallback, useRef, useMemo } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  message,
  Input,
  Button,
  Spin,
  Tooltip,
  Select,
} from 'antd'
import {
  ArrowLeft,
  Save,
  ImagePlus,
  Download,
  Archive,
  Loader,
  Sparkles,
} from 'lucide-react'
import {
  getNote,
  updateNote,
  setArchived,
  uploadImage,
  exportMarkdown,
  listTags,
} from './notes.ts'
import { useSSE } from '../../shared/api/useSSE.ts'
import { NoteItem, useDebounce } from '../../shared/lib/types.ts'
import { useTheme } from '../../theme.tsx'
import { downloadBlob } from '../../shared/lib/utils.ts'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { formatError } from '../../shared/lib/errors.ts'

export default function EditorPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { dark } = useTheme()
  const [note, setNote] = useState<NoteItem | null>(null)
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [tags, setTags] = useState<string[]>([])
  const [tagOptions, setTagOptions] = useState<string[]>([])
  const [saving, setSaving] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [online, setOnline] = useState(navigator.onLine)
  const fileRef = useRef<HTMLInputElement | null>(null)
  const lastSyncRef = useRef<string>('')
  const dirtyRef = useRef(false)
  const [stale, setStale] = useState(false)

  useEffect(() => {
    const on = () => setOnline(true)
    const off = () => setOnline(false)
    window.addEventListener('online', on)
    window.addEventListener('offline', off)
    return () => {
      window.removeEventListener('online', on)
      window.removeEventListener('offline', off)
    }
  }, [])

  const load = useCallback(async () => {
    if (!id) return
    try {
      const n = await getNote(id)
      setNote(n)
      setTitle(n.title)
      setBody(n.body)
      setTags(n.tags ?? [])
      lastSyncRef.current = n.updatedAt
      dirtyRef.current = false
      setStale(false)
    } catch (e: any) {
      message.error(formatError(e, '加载笔记失败'))
    }
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  useEffect(() => {
    listTags()
      .then((rows) => setTagOptions(rows.map((r) => r.name)))
      .catch(() => {})
  }, [])

  const onSSEEvent = useCallback(
    (ev: { type: string; noteId: string }) => {
      if (ev.noteId === id) {
        if (dirtyRef.current) {
          setStale(true)
        } else {
          load()
        }
      }
    },
    [id, load],
  )
  useSSE(onSSEEvent)

  // 最新编辑值的 ref，使 autosave 回调保持稳定引用（不随每次按键重建）
  const latestRef = useRef({ id, title, body, tags })
  latestRef.current = { id, title, body, tags }

  const doAutosave = useCallback(async () => {
    const cur = latestRef.current
    if (!cur.id || !dirtyRef.current) return
    try {
      setSaving(true)
      await updateNote(cur.id, { title: cur.title || '未命名', body: cur.body, tags: cur.tags })
      dirtyRef.current = false
      setStale(false)
    } catch (e: any) {
      message.error(formatError(e, '自动保存失败'))
    } finally {
      setSaving(false)
    }
  }, [])

  // 停笔 2 秒后自动保存；每次编辑重置计时器
  useEffect(() => {
    if (!id || !dirtyRef.current) return
    const t = setTimeout(doAutosave, 2000)
    return () => clearTimeout(t)
  }, [id, title, body, tags, doAutosave])
  const debouncedBody = useDebounce(body, 300)

  const preview = useMemo(() => {
    const html = marked.parse(debouncedBody || '') as string
    return DOMPurify.sanitize(html)
  }, [debouncedBody])

  const handleSave = async () => {
    if (!id) return
    setSaving(true)
    try {
      await updateNote(id, { title: title || '未命名', body, tags })
      dirtyRef.current = false
      setStale(false)
      message.success('已保存')
      load()
    } catch (e: any) {
      message.error(formatError(e, '保存失败'))
    }
    setSaving(false)
  }

  const handleUpload = async (file: File) => {
    if (!id) return
    setUploading(true)
    try {
      const { url } = await uploadImage(file, id)
      const md = `![](${url})`
      setBody((prev) => prev + '\n' + md)
      dirtyRef.current = true
      message.success('图片已上传')
    } catch (e: any) {
      message.error(formatError(e, '上传失败'))
    }
    setUploading(false)
  }

  const handleExport = async () => {
    if (!id) return
    try {
      const blob = await exportMarkdown(id)
      downloadBlob(blob, 'note.md')
    } catch (e: any) {
      message.error(formatError(e, '导出失败'))
    }
  }

  // 把当前笔记交给 AI 助手处理：组装成提示词并跳转到工作模式，输入已预填。
  const handleAiProcess = () => {
    const prompt = `请帮我阅读并优化以下笔记，给出结构化的润色与补充建议：\n\n# ${title || '无标题'}\n\n${body}`
    navigate('/agent', { state: { draft: prompt } })
  }

  if (!note) {
    return (
      <div style={{ display: 'grid', placeItems: 'center', height: '100vh' }}>
        <Spin size="large" />
      </div>
    )
  }

  return (
    <div className="main" style={{ height: '100vh' }}>
      <div className="main-header">
        <Button
          icon={<ArrowLeft size={14} />}
          onClick={() => navigate('/notes')}
          type="text"
        >
          返回
        </Button>
        <Input
         value={title}
          onChange={(e) => {
            setTitle(e.target.value)
            dirtyRef.current = true
          }}
         placeholder="笔记标题"
          style={{ flex: 1, maxWidth: 400, fontWeight: 600 }}
          bordered={false}
        />
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          <Select
            mode="tags"
            value={tags}
            options={tagOptions.map((t) => ({ label: '#' + t, value: t }))}
            onChange={(vals: string[]) => {
              setTags(vals)
              dirtyRef.current = true
            }}
            placeholder="标签"
            style={{ minWidth: 160 }}
            size="small"
            maxTagCount="responsive"
            suffixIcon={null}
          />
          <Tooltip title="让 AI 处理">
            <Button size="small" icon={<Sparkles size={14} />} onClick={handleAiProcess}>
              让 AI 处理
            </Button>
          </Tooltip>
          <Tooltip title="上传图片">
            <Button
              size="small"
              icon={<ImagePlus size={14} />}
              loading={uploading}
              onClick={() => fileRef.current?.click()}
            />
          </Tooltip>
          <input
            ref={fileRef}
            type="file"
            accept="image/*"
            style={{ display: 'none' }}
            onChange={(e) => {
              const f = e.target.files?.[0]
              if (f) handleUpload(f)
            }}
          />
          <Tooltip title={note.archived ? '取消归档' : '归档'}>
            <Button
              size="small"
              icon={<Archive size={14} />}
               onClick={async () => {
                if (!id) return
                await setArchived(id, !note.archived)
                load()
              }}
            />
          </Tooltip>
          <Tooltip title="导出 Markdown">
            <Button size="small" icon={<Download size={14} />} onClick={handleExport} />
          </Tooltip>
          <Button
            size="small"
            type="primary"
            icon={saving ? <Loader size={14} /> : <Save size={14} />}
            loading={saving}
            onClick={handleSave}
          >
            保存
          </Button>
        </div>
      </div>
      <div
        className="editor-layout"
        style={{ padding: 16, flex: 1, minHeight: 0 }}
      >
        {!online && (
          <div
            style={{
              marginBottom: 8,
              padding: '8px 12px',
              borderRadius: 8,
              background: '#fef3c7',
              color: '#92400e',
              fontSize: 13,
            }}
          >
           离线模式：仅可查看，重新连接后自动同步
         </div>
       )}
        {stale && (
          <div
            style={{
              marginBottom: 8,
              padding: '8px 12px',
              borderRadius: 8,
              background: '#ede9fe',
              color: '#5b21b6',
              fontSize: 13,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
            }}
          >
            <span>此笔记已在其他会话更新</span>
            <Button size="small" onClick={() => load()}>
              加载最新
            </Button>
          </div>
        )}
       <div className="editor-pane">
          <textarea
           value={body}
           readOnly={!online}
            onChange={(e) => {
              setBody(e.target.value)
              dirtyRef.current = true
            }}
           placeholder="支持 Markdown 编辑，图片可粘贴或点击上传..."
            onPaste={(e) => {
              const items = e.clipboardData?.items
              if (!items) return
              for (const item of Array.from(items)) {
                if (item.type.startsWith('image/')) {
                  const file = item.getAsFile()
                  if (file) {
                    e.preventDefault()
                    handleUpload(file)
                  }
                }
              }
            }}
          />
        </div>
        <div className="preview-pane" dangerouslySetInnerHTML={{ __html: preview }} />
      </div>
    </div>
  )
}
