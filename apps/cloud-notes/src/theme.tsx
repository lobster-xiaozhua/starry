import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react'

type ThemeCtx = {
  dark: boolean
  toggle: () => void
  setTheme: (dark: boolean) => void
}

const ThemeContext = createContext<ThemeCtx>({
  dark: false,
  toggle: () => {},
  setTheme: () => {},
})

export function useTheme() {
  return useContext(ThemeContext)
}

function initialDark(): boolean {
  const saved = localStorage.getItem('notes-theme')
  if (saved === 'dark') return true
  if (saved === 'light') return false
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [dark, setDark] = useState(initialDark)

  const setTheme = useCallback((d: boolean) => {
    setDark(d)
    localStorage.setItem('notes-theme', d ? 'dark' : 'light')
  }, [])

  const toggle = useCallback(() => {
    setDark((prev) => {
      const next = !prev
      localStorage.setItem('notes-theme', next ? 'dark' : 'light')
      return next
    })
  }, [])

  useEffect(() => {
    const mql = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = (e: MediaQueryListEvent) => {
      if (!localStorage.getItem('notes-theme')) setDark(e.matches)
    }
    mql.addEventListener('change', handler)
    return () => mql.removeEventListener('change', handler)
  }, [])

  return (
    <ThemeContext.Provider value={{ dark, toggle, setTheme }}>{children}</ThemeContext.Provider>
  )
}

export { ThemeContext }
