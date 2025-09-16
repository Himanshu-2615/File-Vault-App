import React from 'react'
import { createRoot } from 'react-dom/client'
import { createClient, Provider, cacheExchange, fetchExchange } from 'urql'
import App from './pages/App'

const client = createClient({
  url: (import.meta as any).env?.VITE_API_URL || 'http://localhost:8080/graphql',
  exchanges: [cacheExchange, fetchExchange],
})

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <Provider value={client}>
      <App />
    </Provider>
  </React.StrictMode>
)


