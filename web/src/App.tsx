import { BrowserRouter, Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import AgentList from './components/AgentList'
import AgentDetail from './components/AgentDetail'

export default function App() {
  return (
    <BrowserRouter>
      <Layout>
        <Routes>
          <Route path="/" element={<AgentList />} />
          <Route path="/agents/:id" element={<AgentDetail />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  )
}
