import { useCallback, useEffect, useRef, useState } from 'react'
import {
  App,
  Button,
  Empty,
  Input,
  List,
  Popconfirm,
  Progress,
  Space,
  Spin,
  Tag,
} from 'antd'
import { Folder, File as FileIcon, Upload, Trash2, Download, Pencil, FolderPlus } from 'lucide-react'
import {
  createFolder,
  deleteDrive,
  downloadFile,
  listDrive,
  renameFile,
  uploadFile,
} from './drive.ts'
import type { DriveFile } from '../../shared/lib/types.ts'

type Crumb = { id: string; name: string }

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`
  return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`
}

export default function DrivePage() {
  const { message } = App.useApp()
  const [path, setPath] = useState<Crumb[]>([])
  const [items, setItems] = useState<DriveFile[]>([])
  const [used, setUsed] = useState(0)
  const [quota, setQuota] = useState(0)
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [dragOver, setDragOver] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  const parentId = path.length ? path[path.length - 1].id : 'root'

  const reload = useCallback(async () => {
    try {
      const data = await listDrive(parentId)
      setItems(data.items)
      setUsed(data.used)
      setQuota(data.quota)
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || '加载失败')
    } finally {
      setLoading(false)
    }
  }, [message, parentId])

  useEffect(() => {
    setLoading(true)
    reload()
  }, [reload])

  const handleUpload = async (files: FileList | null) => {
    if (!files || files.length === 0) return
    setUploading(true)
    try {
      for (const f of Array.from(files)) {
        await uploadFile(f, parentId)
      }
      message.success('上传完成')
      reload()
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || '上传失败')
    } finally {
      setUploading(false)
    }
  }

  const handleNewFolder = async () => {
    const name = window.prompt('文件夹名称', '新建文件夹')
    if (!name || !name.trim()) return
    try {
      await createFolder(name.trim(), parentId)
      reload()
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || '创建失败')
    }
  }

  const handleRename = async (f: DriveFile) => {
    const name = window.prompt('新名称', f.name)
    if (!name || !name.trim()) return
    try {
      await renameFile(f.id, name.trim())
      reload()
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || '改名失败')
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await deleteDrive(id)
      message.success('已删除')
      reload()
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || '删除失败')
    }
  }

  const openFolder = (f: DriveFile) => {
    setPath((p) => [...p, { id: f.id, name: f.name }])
  }

  const jumpTo = (idx: number) => {
    setPath((p) => p.slice(0, idx + 1))
  }

  const usedPct = quota > 0 ? Math.min(100, Math.round((used / quota) * 100)) : 0

  return (
    <div style={{ padding: 24, maxWidth: 1000, margin: '0 auto' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12, flexWrap: 'wrap', gap: 8 }}>
        <Space wrap>
          <span style={{ color: 'var(--text-secondary)' }}>位置：</span>
          <Button type="link" size="small" style={{ padding: 0 }} onClick={() => setPath([])}>
            根目录
          </Button>
          {path.map((c, i) => (
            <span key={c.id}>
              <span style={{ color: 'var(--text-secondary)' }}> / </span>
              <Button type="link" size="small" style={{ padding: 0 }} onClick={() => jumpTo(i)}>
                {c.name}
              </Button>
            </span>
          ))}
        </Space>
        <Space>
          <Button icon={<FolderPlus size={16} />} onClick={handleNewFolder}>
            新建文件夹
          </Button>
          <Button type="primary" icon={<Upload size={16} />} loading={uploading} onClick={() => fileRef.current?.click()}>
            上传
          </Button>
        </Space>
      </div>

      <Progress
        percent={usedPct}
        format={() => `${formatBytes(used)} / ${formatBytes(quota)}`}
        style={{ marginBottom: 16 }}
      />

      <div
        onDragOver={(e) => {
          e.preventDefault()
          setDragOver(true)
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={(e) => {
          e.preventDefault()
          setDragOver(false)
          handleUpload(e.dataTransfer.files)
        }}
        style={{
          border: dragOver ? '2px dashed var(--accent)' : '2px dashed transparent',
          borderRadius: 12,
          padding: 8,
        }}
      >
        <input
          ref={fileRef}
          type="file"
          multiple
          style={{ display: 'none' }}
          onChange={(e) => {
            handleUpload(e.target.files)
            e.target.value = ''
          }}
        />
        {loading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin />
          </div>
        ) : items.length === 0 ? (
          <Empty description="空文件夹，拖拽文件到此处或点“上传”" style={{ padding: 40 }} />
        ) : (
          <List
            grid={{ gutter: 12, xs: 1, sm: 2, md: 3, lg: 4 }}
            dataSource={items}
            renderItem={(f) => (
              <List.Item>
                <div
                  onDoubleClick={() => f.isDir && openFolder(f)}
                  style={{
                    border: '1px solid var(--border)',
                    borderRadius: 10,
                    padding: 12,
                    background: 'var(--card)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 6,
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    {f.isDir ? (
                      <Folder size={20} color="#f59e0b" />
                    ) : (
                      <FileIcon size={20} color="var(--accent)" />
                    )}
                    <span
                      style={{
                        flex: 1,
                        color: 'var(--text)',
                        fontWeight: 500,
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {f.name}
                    </span>
                  </div>
                  <div style={{ fontSize: 12, color: 'var(--text-secondary)' }}>
                    {f.isDir ? '文件夹' : formatBytes(f.size)}
                  </div>
                  <Space size={4}>
                    {f.isDir ? (
                      <Button size="small" type="text" icon={<Folder size={14} />} onClick={() => openFolder(f)} />
                    ) : (
                      <Button
                        size="small"
                        type="text"
                        icon={<Download size={14} />}
                        onClick={() => downloadFile(f.id, f.name)}
                      />
                    )}
                    <Button size="small" type="text" icon={<Pencil size={14} />} onClick={() => handleRename(f)} />
                    <Popconfirm title="确认删除？" onConfirm={() => handleDelete(f.id)}>
                      <Button size="small" type="text" danger icon={<Trash2 size={14} />} />
                    </Popconfirm>
                  </Space>
                </div>
              </List.Item>
            )}
          />
        )}
      </div>
    </div>
  )
}
