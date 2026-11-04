import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { HashRouter } from 'react-router-dom'
import { TooltipProvider } from '@/components/ui/tooltip'
import { App } from './App'
import './index.css'

const queryClient = new QueryClient({ defaultOptions: { queries: { refetchOnWindowFocus: false } } })
createRoot(document.getElementById('root')!).render(
  <StrictMode><QueryClientProvider client={queryClient}><HashRouter><TooltipProvider><App /></TooltipProvider></HashRouter></QueryClientProvider></StrictMode>,
)
