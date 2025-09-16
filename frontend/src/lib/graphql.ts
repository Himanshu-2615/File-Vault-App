import { cacheExchange, createClient, fetchExchange } from 'urql'

export function getUserId(): string | null {
  return localStorage.getItem('userId')
}

export const client = createClient({
  url: (import.meta as any).env?.VITE_API_URL || 'http://localhost:8080/graphql',
  exchanges: [cacheExchange, fetchExchange],
  fetchOptions: () => {
    const uid = getUserId()
    return {
      headers: uid ? { 'X-User-ID': uid } : undefined,
    }
  },
})



