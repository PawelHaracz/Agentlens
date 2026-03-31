import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react'
import * as api from '../api'
import type { User } from '../api'

interface AuthContextType {
  user: User | null
  isAuthenticated: boolean
  isLoading: boolean
  permissions: string[]
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  hasPermission: (perm: string) => boolean
  refreshUser: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const permissions = user?.role?.permissions ?? []

  const refreshUser = useCallback(async () => {
    try {
      const me = await api.getMe()
      setUser(me)
    } catch {
      setUser(null)
      api.setToken(null)
    }
  }, [])

  useEffect(() => {
    refreshUser().finally(() => setIsLoading(false))
  }, [refreshUser])

  const loginFn = async (username: string, password: string) => {
    const res = await api.login(username, password)
    api.setToken(res.token)
    setUser(res.user)
  }

  const logoutFn = async () => {
    try { await api.logout() } catch { /* ignore logout errors */ }
    api.setToken(null)
    setUser(null)
  }

  const hasPermission = (perm: string) => permissions.includes(perm)

  return (
    <AuthContext.Provider value={{
      user, isAuthenticated: !!user, isLoading, permissions,
      login: loginFn, logout: logoutFn, hasPermission, refreshUser,
    }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
