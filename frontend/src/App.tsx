import { useEffect, useState } from 'react'
import { HistoryPage } from './HistoryPage.tsx'
import { TimerPage } from './TimerPage.tsx'
import './App.css'

function useRoute(): string {
  const read = () => {
    const raw = window.location.hash.replace(/^#/, '') || '/'
    return raw.startsWith('/') ? raw : `/${raw}`
  }
  const [route, setRoute] = useState(read)
  useEffect(() => {
    const onHash = () => setRoute(read())
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])
  return route
}

export default function App() {
  const route = useRoute()
  if (route.startsWith('/gecmis')) {
    return <HistoryPage />
  }
  return <TimerPage />
}
