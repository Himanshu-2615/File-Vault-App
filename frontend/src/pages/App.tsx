import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Provider, useQuery } from 'urql'
import { client } from '../lib/graphql'

const MY_FILES = `
  query MyFiles($limit: Int, $offset: Int, $nameLike: String, $mimeTypes: [String!], $sizeMin: Int, $sizeMax: Int, $dateFrom: String, $dateTo: String, $tags: [String!]){
    myFiles(limit: $limit, offset: $offset, nameLike: $nameLike, mimeTypes: $mimeTypes, sizeMin: $sizeMin, sizeMax: $sizeMax, dateFrom: $dateFrom, dateTo: $dateTo, tags: $tags){ id filename sizeBytes mimeType isPublic createdAt publicToken downloadCount }
  }
`

const CREATE_LINK = `mutation($fileId: String!){ createPublicLink(fileId: $fileId) }`

const API_URL = (import.meta as any).env.VITE_API_URL || 'http://localhost:8081/graphql'
const API_BASE = API_URL.replace('/graphql', '')

function Uploader(): JSX.Element {
  const [dragOver, setDragOver] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const [message, setMessage] = useState<string | null>(null)

  const onFiles = useCallback(async (files: FileList | null) => {
    if (!files || files.length === 0) return
    const uid = localStorage.getItem('userId') || (Date.now().toString())
    localStorage.setItem('userId', uid)
    const form = new FormData()
    Array.from(files).forEach(f => form.append('files', f))
    try {
      const res = await fetch(API_BASE + '/upload', {
        method: 'POST',
        headers: { 'X-User-ID': uid },
        body: form,
      })
      if (!res.ok) throw new Error('upload failed')
      setMessage('Uploaded successfully')
    } catch (e: any) {
      setMessage(e.message || 'Upload error')
    }
  }, [])

  return (
    <div>
      <div
        onDragOver={e => { e.preventDefault(); setDragOver(true) }}
        onDragLeave={() => setDragOver(false)}
        onDrop={e => { e.preventDefault(); setDragOver(false); onFiles(e.dataTransfer.files) }}
        onClick={() => inputRef.current?.click()}
        style={{
          border: '2px dashed #888', padding: 24, borderRadius: 8, cursor: 'pointer',
          background: dragOver ? '#f0f0f0' : 'transparent'
        }}
      >
        <strong>Drag & drop</strong> files here, or click to select
      </div>
      <input ref={inputRef} type="file" multiple style={{ display: 'none' }} onChange={e => onFiles(e.target.files)} />
      {message && <p>{message}</p>}
    </div>
  )
}

function FilesList(): JSX.Element {
  const [filters, setFilters] = useState({ nameLike: '', mime: '', sizeMin: '', sizeMax: '' })
  const variables = {
    limit: 50, offset: 0,
    nameLike: filters.nameLike || null,
    mimeTypes: filters.mime ? [filters.mime] : null,
    sizeMin: filters.sizeMin ? Number(filters.sizeMin) : null,
    sizeMax: filters.sizeMax ? Number(filters.sizeMax) : null,
  }
  const [result, reexec] = useQuery({ query: MY_FILES, variables })
  const { data, fetching, error } = result

  useEffect(() => {
    const t = setInterval(() => reexec({ requestPolicy: 'network-only' }), 3000)
    return () => clearInterval(t)
  }, [reexec, variables])

  if (fetching) return <p>Loading...</p>
  if (error) return <p>Error: {error.message}</p>
  const files = data?.myFiles || []

  return (
    <div>
      <div style={{ display: 'flex', gap: 8, marginBottom: 12 }}>
        <input placeholder="Search name" value={filters.nameLike} onChange={e => setFilters(f => ({ ...f, nameLike: e.target.value }))} />
        <input placeholder="MIME e.g. image/png" value={filters.mime} onChange={e => setFilters(f => ({ ...f, mime: e.target.value }))} />
        <input placeholder="Min size (bytes)" value={filters.sizeMin} onChange={e => setFilters(f => ({ ...f, sizeMin: e.target.value }))} />
        <input placeholder="Max size (bytes)" value={filters.sizeMax} onChange={e => setFilters(f => ({ ...f, sizeMax: e.target.value }))} />
        <button onClick={() => reexec({ requestPolicy: 'network-only' })}>Apply</button>
      </div>
      <h3>Your files ({files.length})</h3>
      <ul>
        {files.map((f: any) => (
          <li key={f.id}>
            {f.filename} — {Math.round(f.sizeBytes/1024)} KB — {f.mimeType || 'n/a'} — {new Date(f.createdAt).toLocaleString()}
            {f.publicToken ? (
              <>
                {' '}| <a href={API_BASE + '/d/' + f.publicToken} target="_blank">Public link</a>
                {' '}({f.downloadCount} downloads)
              </>
            ) : (
              <>
                {' '}| <button onClick={async () => {
                  await fetch(API_URL, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json', 'X-User-ID': localStorage.getItem('userId')||'' },
                    body: JSON.stringify({ query: CREATE_LINK, variables: { fileId: f.id } })
                  })
                  reexec({ requestPolicy: 'network-only' })
                }}>Make public</button>
              </>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}

const STORAGE_STATS = `
  query { myStorageStats { originalBytes dedupedBytes savedBytes savedPercent } }
`

function StorageStats(): JSX.Element {
  const [{ data, fetching, error }] = useQuery({ query: STORAGE_STATS })
  if (fetching) return <p>Calculating...</p>
  if (error) return <p>Error loading stats</p>
  const s = data?.myStorageStats
  if (!s) return <></>
  return (
    <div style={{ marginTop: 12 }}>
      <h3>Storage</h3>
      <div>Original: {s.originalBytes} B</div>
      <div>Deduped: {s.dedupedBytes} B</div>
      <div>Savings: {s.savedBytes} B ({s.savedPercent.toFixed(2)}%)</div>
    </div>
  )
}

const ALL_FILES = `query { allFiles { id filename sizeBytes mimeType isPublic createdAt publicToken downloadCount } }`
const ALL_USERS = `query { allUsers { id email name role createdAt } }`
const SET_ROLE = `mutation($userId: String!, $role: String!){ setUserRole(userId: $userId, role: $role) }`

function AdminPanel(): JSX.Element {
  const [files, setFiles] = useState<any[]>([])
  const [users, setUsers] = useState<any[]>([])
  const [loading, setLoading] = useState(false)

  const loadData = async () => {
    setLoading(true)
    try {
      const [filesRes, usersRes] = await Promise.all([
        fetch(API_URL, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'X-User-ID': localStorage.getItem('userId')||'' },
          body: JSON.stringify({ query: ALL_FILES })
        }).then(r => r.json()),
        fetch(API_URL, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'X-User-ID': localStorage.getItem('userId')||'' },
          body: JSON.stringify({ query: ALL_USERS })
        }).then(r => r.json())
      ])
      setFiles(filesRes.data?.allFiles || [])
      setUsers(usersRes.data?.allUsers || [])
    } catch (e) { console.error(e) }
    setLoading(false)
  }

  useEffect(() => { loadData() }, [])

  if (loading) return <p>Loading admin data...</p>

  return (
    <div style={{ marginTop: 24, border: '1px solid #ccc', padding: 16 }}>
      <h2>Admin Panel</h2>
      <div style={{ display: 'flex', gap: 16 }}>
        <div style={{ flex: 1 }}>
          <h3>All Files ({files.length})</h3>
          <ul style={{ maxHeight: 200, overflow: 'auto' }}>
            {files.map(f => (
              <li key={f.id}>{f.filename} — {f.sizeBytes} B — {f.downloadCount} downloads</li>
            ))}
          </ul>
        </div>
        <div style={{ flex: 1 }}>
          <h3>All Users ({users.length})</h3>
          <ul style={{ maxHeight: 200, overflow: 'auto' }}>
            {users.map(u => (
              <li key={u.id}>
                {u.name} ({u.email}) — {u.role}
                <button onClick={async () => {
                  await fetch(API_URL, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json', 'X-User-ID': localStorage.getItem('userId')||'' },
                    body: JSON.stringify({ query: SET_ROLE, variables: { userId: u.id, role: u.role === 'admin' ? 'user' : 'admin' } })
                  })
                  loadData()
                }}>Toggle Admin</button>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </div>
  )
}

export default function App(): JSX.Element {
  useEffect(() => {
    if (!localStorage.getItem('userId')) localStorage.setItem('userId', Date.now().toString())
  }, [])
  return (
    <Provider value={client}>
      <div style={{ fontFamily: 'sans-serif', padding: 16 }}>
        <h1>File Vault</h1>
        <Uploader />
        <StorageStats />
        <FilesList />
        <AdminPanel />
      </div>
    </Provider>
  )
}
