import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AuthProvider } from './contexts/AuthContext'
import { ThemeProvider } from './contexts/ThemeContext'
import Layout from './components/Layout'
import ProtectedRoute from './components/ProtectedRoute'
import CatalogListPage from './routes/catalog/CatalogListPage'
import CatalogDetailPage from './routes/catalog/CatalogDetailPage'
import CapabilityListPage from './routes/capabilities/CapabilityListPage'
import CapabilityDetailPage from './routes/capabilities/CapabilityDetailPage'
import LoginPage from './pages/LoginPage'
import SettingsPage from './pages/SettingsPage'

export default function App() {
  return (
    <BrowserRouter>
      <ThemeProvider>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/" element={<ProtectedRoute><Layout><CatalogListPage /></Layout></ProtectedRoute>} />
            <Route path="/catalog/capabilities" element={<ProtectedRoute><Layout><CapabilityListPage /></Layout></ProtectedRoute>} />
            <Route path="/catalog/capabilities/:key" element={<ProtectedRoute><Layout><CapabilityDetailPage /></Layout></ProtectedRoute>} />
            <Route path="/catalog/:id" element={<ProtectedRoute><Layout><CatalogDetailPage /></Layout></ProtectedRoute>} />
            <Route path="/settings" element={<ProtectedRoute><Layout><SettingsPage /></Layout></ProtectedRoute>} />
          </Routes>
        </AuthProvider>
      </ThemeProvider>
    </BrowserRouter>
  )
}
