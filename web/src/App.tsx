import { BrowserRouter, Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import CatalogList from './components/CatalogList'
import EntryDetail from './components/EntryDetail'

export default function App() {
  return (
    <BrowserRouter>
      <Layout>
        <Routes>
          <Route path="/" element={<CatalogList />} />
          <Route path="/catalog/:id" element={<EntryDetail />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  )
}
