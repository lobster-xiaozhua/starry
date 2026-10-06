import { useCallback, useEffect, useState } from 'react'
import { api } from '@shared/api'

export function useCaptcha() {
  const [captchaId, setCaptchaId] = useState('')
  const [image, setImage] = useState('')
  const [code, setCode] = useState('')
  const [enabled, setEnabled] = useState(false)
  const [loading, setLoading] = useState(true)

  const reload = useCallback(async () => {
    setLoading(true)
    try {
      const res = await api.post<{ code: number; data: { enabled: boolean; captchaId?: string; image?: string } }>(
        '/auth/captcha',
      )
      if (res.data.data.enabled && res.data.data.captchaId) {
        setEnabled(true)
        setCaptchaId(res.data.data.captchaId)
        setImage(res.data.data.image ?? '')
      } else {
        setEnabled(false)
        setCaptchaId('')
        setImage('')
      }
    } catch {
      setEnabled(false)
      setCaptchaId('')
      setImage('')
    }
    setCode('')
    setLoading(false)
  }, [])

  useEffect(() => {
    void reload()
  }, [reload])

  return { captchaId, image, code, setCode, enabled, loading, reload }
}
