import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Alert,
  App,
  Button,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Tag,
  Typography,
} from 'antd'
import { KeyRound, Plus, Trash2, Eye, EyeOff, Lock, ShieldAlert } from 'lucide-react'
import {
  createVaultItem,
  deleteVaultItem,
  getVaultSalt,
  listVaultItems,
  setupVault,
  updateVaultItem,
} from '../api/vault'
import {
  decryptJSON,
  deriveKey,
  encryptJSON,
  makeVerifier,
  verifyKey,
} from '../crypto'
import type { VaultItem, VaultType } from '../types'

const { Text } = Typography

type Phase = 'checking' | 'setup' | 'locked' | 'unlocked'

const TYPE_META: Record<VaultType, { color: string; text: string }> = {
  password: { color: 'blue', text: '账号密码' },
  note: { color: 'default', text: '私密笔记' },
  card: { color: 'purple', text: '银行卡' },
  apikey: { color: 'green', text: 'API 密钥' },
}

type FormState = {
  id?: string
  title: string
  type: VaultType
  fields: Record<string, string>
}

const blankForm = (): FormState => ({ title: '', type: 'password', fields: {} })

function fieldsFor(type: VaultType): { key: string; label: string; secret?: boolean }[] {
  switch (type) {
    case 'password':
      return [
        { key: 'username', label: '账号' },
        { key: 'password', label: '密码', secret: true },
        { key: 'url', label: '网址' },
        { key: 'note', label: '备注' },
      ]
    case 'note':
      return [{ key: 'note', label: '内容' }]
    case 'card':
      return [
        { key: 'number', label: '卡号', secret: true },
        { key: 'holder', label: '持卡人' },
        { key: 'expiry', label: '有效期' },
        { key: 'cvv', label: 'CVV', secret: true },
        { key: 'note', label: '备注' },
      ]
    case 'apikey':
      return [
        { key: 'key', label: '密钥', secret: true },
        { key: 'note', label: '备注' },
      ]
  }
}

export default function VaultPage() {
  const { message } = App.useApp()
  const [phase, setPhase] = useState<Phase>('checking')
  const [salt, setSalt] = useState('')
  const [verifier, setVerifier] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPw, setConfirmPw] = useState('')
  const [pwError, setPwError] = useState('')
  const [items, setItems] = useState<VaultItem[]>([])
  const [revealed, setRevealed] = useState<Record<string, any>>({})
  const [modalOpen, setModalOpen] = useState(false)
  const [form, setForm] = useState<FormState>(blankForm())
  const [busy, setBusy] = useState(false)

  // 密钥仅存于内存（ref），绝不持久化；刷新/锁定即失效。
  const keyRef = useRef<CryptoKey | null>(null)

  const loadSalt = useCallback(async () => {
    try {
      const data = await getVaultSalt()
      setSalt(data.salt)
      setVerifier(data.verifier)
      setPhase(data.setup ? 'locked' : 'setup')
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || '加载失败')
      setPhase('locked')
    }
  }, [message])

  useEffect(() => {
    loadSalt()
  }, [loadSalt])

  const loadItems = useCallback(async () => {
    try {
      setItems(await listVaultItems())
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || '加载失败')
    }
  }, [message])

  const doSetup = async () => {
    if (!password) {
      setPwError('请输入主密码')
      return
    }
    if (password !== confirmPw) {
      setPwError('两次输入不一致')
      return
    }
    setBusy(true)
    try {
      const key = await deriveKey(password, salt)
      const v = await makeVerifier(key)
      await setupVault(v)
      keyRef.current = key
      setPhase('unlocked')
      setPassword('')
      setConfirmPw('')
      await loadItems()
      message.success('保险箱已创建并解锁')
    } catch (e: any) {
      message.error(e?.message || '创建失败')
    } finally {
      setBusy(false)
    }
  }

  const doUnlock = async () => {
    if (!password) {
      setPwError('请输入主密码')
      return
    }
    setBusy(true)
    try {
      const key = await deriveKey(password, salt)
      const ok = await verifyKey(key, verifier)
      if (!ok) {
        setPwError('主密码错误')
        setBusy(false)
        return
      }
      keyRef.current = key
      setPhase('unlocked')
      setPassword('')
      await loadItems()
    } catch {
      setPwError('主密码错误')
    } finally {
      setBusy(false)
    }
  }

  const lock = () => {
    keyRef.current = null
    setRevealed({})
    setPassword('')
    setPhase('locked')
  }

  const openCreate = () => {
    setForm(blankForm())
    setModalOpen(true)
  }
  const openEdit = (it: VaultItem) => {
    setForm({ id: it.id, title: it.title, type: it.type as VaultType, fields: {} })
    setModalOpen(true)
  }

  const handleSave = async () => {
    if (!keyRef.current) return
    if (!form.title.trim()) {
      message.warning('请输入标题')
      return
    }
    setBusy(true)
    try {
      const encrypted = await encryptJSON(keyRef.current, { type: form.type, fields: form.fields })
      if (form.id) {
        await updateVaultItem(form.id, { title: form.title.trim(), encrypted })
      } else {
        await createVaultItem({ title: form.title.trim(), type: form.type, encrypted })
      }
      setModalOpen(false)
      await loadItems()
      message.success('已保存')
    } catch (e: any) {
      message.error(e?.message || '保存失败')
    } finally {
      setBusy(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await deleteVaultItem(id)
      setRevealed((r) => {
        const n = { ...r }
        delete n[id]
        return n
      })
      await loadItems()
      message.success('已删除')
    } catch {
      message.error('删除失败')
    }
  }

  const toggleReveal = async (it: VaultItem) => {
    if (!keyRef.current) return
    if (revealed[it.id]) {
      setRevealed((r) => {
        const n = { ...r }
        delete n[it.id]
        return n
      })
      return
    }
    try {
      const obj = await decryptJSON<{ type: VaultType; fields: Record<string, string> }>(
        keyRef.current,
        it.encrypted,
      )
      setRevealed((r) => ({ ...r, [it.id]: obj.fields }))
    } catch {
      message.error('解密失败（主密码可能已变更）')
    }
  }

  if (phase === 'checking') {
    return <div style={{ padding: 48, textAlign: 'center', color: 'var(--text-secondary)' }}>加载中…</div>
  }

  if (phase === 'setup' || phase === 'locked') {
    const isSetup = phase === 'setup'
    return (
      <div style={{ maxWidth: 420, margin: '8vh auto', padding: 24 }}>
        <Space direction="vertical" size={14} style={{ width: '100%' }}>
          <div style={{ textAlign: 'center' }}>
            <KeyRound size={32} color="var(--accent)" />
            <h2 style={{ margin: '12px 0 4px', color: 'var(--text)' }}>
              {isSetup ? '设置保险箱主密码' : '解锁保险箱'}
            </h2>
            <Text type="secondary">
              {isSetup ? '首次使用，请设定主密码' : '输入主密码以解密你的密项'}
            </Text>
          </div>

          <Alert
            type="warning"
            showIcon
            icon={<ShieldAlert size={16} />}
            message="主密码仅保存在你本地，永不上传。遗忘将无法恢复已存密项，请务必牢记。"
          />

          <Input.Password
            placeholder="主密码"
            value={password}
            onChange={(e) => {
              setPassword(e.target.value)
              setPwError('')
            }}
            onPressEnter={isSetup ? doSetup : doUnlock}
          />
          {isSetup && (
            <Input.Password
              placeholder="确认主密码"
              value={confirmPw}
              onChange={(e) => setConfirmPw(e.target.value)}
              onPressEnter={doSetup}
            />
          )}
          {pwError && <Text type="danger">{pwError}</Text>}

          {phase === 'locked' && (
            <Button type="link" size="small" onClick={loadSalt} style={{ padding: 0 }}>
              重新检查保险箱状态
            </Button>
          )}

          <Button
            type="primary"
            block
            loading={busy}
            icon={<Lock size={16} />}
            onClick={isSetup ? doSetup : doUnlock}
          >
            {isSetup ? '创建并解锁' : '解锁'}
          </Button>
        </Space>
      </div>
    )
  }

  // unlocked
  return (
    <div style={{ padding: 24, maxWidth: 900, margin: '0 auto' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 14 }}>
        <h2 style={{ margin: 0, color: 'var(--text)' }}>保险箱</h2>
        <Space>
          <Button icon={<Plus size={16} />} type="primary" onClick={openCreate}>
            添加密项
          </Button>
          <Button icon={<Lock size={16} />} onClick={lock}>
            锁定
          </Button>
        </Space>
      </div>

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 14 }}
        message="零知识加密：服务器仅保存密文，主密码与明文不出本地。离开请点“锁定”。"
      />

      {items.length === 0 ? (
        <div style={{ textAlign: 'center', color: 'var(--text-secondary)', padding: 60 }}>
          还没有密项，点“添加密项”开始。
        </div>
      ) : (
        <Space direction="vertical" size={10} style={{ width: '100%' }}>
          {items.map((it) => (
            <div
              key={it.id}
              style={{
                border: '1px solid var(--border)',
                borderRadius: 12,
                padding: 14,
                background: 'var(--card)',
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Space>
                  <strong style={{ color: 'var(--text)' }}>{it.title}</strong>
                  <Tag color={TYPE_META[it.type as VaultType]?.color}>
                    {TYPE_META[it.type as VaultType]?.text}
                  </Tag>
                </Space>
                <Space size={4}>
                  <Button
                    size="small"
                    type="text"
                    icon={revealed[it.id] ? <EyeOff size={14} /> : <Eye size={14} />}
                    onClick={() => toggleReveal(it)}
                  />
                  <Button size="small" type="text" icon={<Plus size={14} />} onClick={() => openEdit(it)} />
                  <Popconfirm title="确认删除该密项？" onConfirm={() => handleDelete(it.id)}>
                    <Button size="small" type="text" danger icon={<Trash2 size={14} />} />
                  </Popconfirm>
                </Space>
              </div>
              {revealed[it.id] && (
                <div style={{ marginTop: 10, display: 'flex', flexDirection: 'column', gap: 4 }}>
                  {Object.entries(revealed[it.id] as Record<string, string>).map(([k, v]) => (
                    <div key={k} style={{ fontSize: 13 }}>
                      <Text type="secondary">{k}：</Text>
                      <Text copyable style={{ color: 'var(--text)' }}>
                        {v || '（空）'}
                      </Text>
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))}
        </Space>
      )}

      <Modal
        open={modalOpen}
        title={form.id ? '编辑密项' : '添加密项'}
        onOk={handleSave}
        onCancel={() => setModalOpen(false)}
        confirmLoading={busy}
        okText="保存"
        cancelText="取消"
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Input
            placeholder="标题，如 GitHub、银行卡"
            value={form.title}
            onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))}
          />
          <Select
            value={form.type}
            style={{ width: '100%' }}
            onChange={(v) => setForm((f) => ({ ...f, type: v, fields: {} }))}
            options={[
              { value: 'password', label: '账号密码' },
              { value: 'note', label: '私密笔记' },
              { value: 'card', label: '银行卡' },
              { value: 'apikey', label: 'API 密钥' },
            ]}
          />
          {fieldsFor(form.type).map((f) => (
            <Input
              key={f.key}
              placeholder={f.label}
              type={f.secret ? 'password' : 'text'}
              value={form.fields[f.key] ?? ''}
              onChange={(e) =>
                setForm((s) => ({ ...s, fields: { ...s.fields, [f.key]: e.target.value } }))
              }
            />
          ))}
        </Space>
      </Modal>
    </div>
  )
}
