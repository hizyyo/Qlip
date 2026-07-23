import { useState, useEffect, useRef, useCallback } from 'react'
import './App.css'

interface HistoryItem {
  id: number
  content_type: string
  content: string
  is_favorite: boolean
  created_at: string
  thumbnail?: string
}

interface Snippet {
  id: number
  title: string
  content: string
  created_at: string
}

type Tab = 'all' | 'text' | 'image' | 'favorites' | 'snippets'

export default function App() {
  const [items, setItems] = useState<HistoryItem[]>([])
  const [snippets, setSnippets] = useState<Snippet[]>([])
  const [tab, setTab] = useState<Tab>('all')
  const [query, setQuery] = useState('')
  const [snippetTitle, setSnippetTitle] = useState('')
  const [snippetContent, setSnippetContent] = useState('')
  const [showSnippetForm, setShowSnippetForm] = useState(false)
  const searchRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    searchRef.current?.focus()
  }, [])

  const dragRef = useRef({ dragging: false, startX: 0, startY: 0, winX: 0, winY: 0 })

  const onHeaderMouseDown = useCallback((e: React.MouseEvent) => {
    const drag = dragRef.current
    drag.dragging = true
    drag.startX = e.screenX
    drag.startY = e.screenY
  }, [])

  useEffect(() => {
    const onMouseMove = (e: MouseEvent) => {
      const drag = dragRef.current
      if (!drag.dragging) return
      const dx = e.screenX - drag.startX
      const dy = e.screenY - drag.startY
      if ((window as any).__moveWindow) {
        ;(window as any).__moveWindow(drag.winX + dx, drag.winY + dy)
      }
    }
    const onMouseUp = (e: MouseEvent) => {
      const drag = dragRef.current
      if (!drag.dragging) return
      drag.dragging = false
      const dx = e.screenX - drag.startX
      const dy = e.screenY - drag.startY
      drag.winX += dx
      drag.winY += dy
    }
    window.addEventListener('mousemove', onMouseMove)
    window.addEventListener('mouseup', onMouseUp)
    return () => {
      window.removeEventListener('mousemove', onMouseMove)
      window.removeEventListener('mouseup', onMouseUp)
    }
  }, [])

  const fetchHistory = useCallback(async (type = 'all') => {
    try {
      const url = type === 'all' ? `/api/history` : `/api/history?type=${type}`
      const res = await fetch(url)
      const data = await res.json()
      setItems(data)
    } catch {}
  }, [])

  const fetchSnippets = useCallback(async () => {
    try {
      const res = await fetch(`/api/snippets`)
      const data = await res.json()
      setSnippets(data)
    } catch {}
  }, [])

  useEffect(() => {
    if (tab === 'snippets') {
      fetchSnippets()
    } else {
      fetchHistory(tab === 'favorites' ? 'favorites' : tab === 'text' ? 'text' : tab === 'image' ? 'image' : '')
    }
  }, [tab, fetchHistory, fetchSnippets])

  useEffect(() => {
    const interval = setInterval(() => {
      if (tab !== 'snippets') {
        fetchHistory(tab === 'favorites' ? 'favorites' : tab === 'text' ? 'text' : tab === 'image' ? 'image' : '')
      }
    }, 2000)
    return () => clearInterval(interval)
  }, [tab, fetchHistory])

  const handleSearch = useCallback(async (q: string) => {
    setQuery(q)
    if (!q.trim()) {
      fetchHistory(tab === 'favorites' ? 'favorites' : tab === 'text' ? 'text' : tab === 'image' ? 'image' : '')
      return
    }
    try {
      const res = await fetch(`/api/search?q=${encodeURIComponent(q)}&limit=50`)
      const data = await res.json()
      setItems(data)
    } catch {}
  }, [tab, fetchHistory])

  const toggleFavorite = useCallback(async (id: number) => {
    try {
      const res = await fetch(`/api/favorite?id=${id}`, { method: 'POST' })
      const data = await res.json()
      setItems(prev => prev.map(i => i.id === id ? { ...i, is_favorite: data.favorite } : i))
    } catch {}
  }, [])

  const deleteItem = useCallback(async (id: number) => {
    try {
      await fetch(`/api/delete?id=${id}`, { method: 'POST' })
      setItems(prev => prev.filter(i => i.id !== id))
    } catch {}
  }, [])

  const deleteSnippet = useCallback(async (id: number) => {
    try {
      await fetch(`/api/snippets?id=${id}`, { method: 'DELETE' })
      setSnippets(prev => prev.filter(s => s.id !== id))
    } catch {}
  }, [])

  const copyToClipboard = useCallback(async (item: HistoryItem) => {
    try {
      const body: Record<string, string> = {}
      if (item.content_type === 'image' && item.thumbnail) {
        body.image_path = item.content
      } else if (item.content) {
        body.text = item.content
      } else {
        return
      }
      await fetch(`/api/copy`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
    } catch {}
  }, [])

  const addSnippet = useCallback(async () => {
    if (!snippetTitle.trim() || !snippetContent.trim()) return
    try {
      await fetch(`/api/snippets`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: snippetTitle, content: snippetContent }),
      })
      setSnippetTitle('')
      setSnippetContent('')
      setShowSnippetForm(false)
      fetchSnippets()
    } catch {}
  }, [snippetTitle, snippetContent, fetchSnippets])

  const formatTime = (iso: string) => {
    const d = new Date(iso)
    const now = new Date()
    const diff = now.getTime() - d.getTime()
    if (diff < 60000) return 'just now'
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`
    return d.toLocaleDateString()
  }

  const copyText = useCallback(async (text: string) => {
    try {
      await fetch(`/api/copy`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text }),
      })
    } catch {}
  }, [])

  return (
    <div className="app">
      <div className="window-header" onMouseDown={onHeaderMouseDown}>
        <div className="title-bar">
          <svg className="logo" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <rect x="9" y="3" width="6" height="4" rx="1" />
            <path d="M5 8h14a1 1 0 0 1 1 1v10a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V9a1 1 0 0 1 1-1z" />
          </svg>
          <span className="title">ClipFlow</span>
        </div>
        <button className="close-btn" onClick={() => window.__toggle?.()}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
            <path d="M18 6L6 18M6 6l12 12" />
          </svg>
        </button>
      </div>

      <div className="search-bar">
        <svg className="search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
          <circle cx="11" cy="11" r="8" />
          <path d="m21 21-4.35-4.35" />
        </svg>
        <input
          ref={searchRef}
          type="text"
          className="search-input"
          placeholder="Search clipboard..."
          value={query}
          onChange={e => handleSearch(e.target.value)}
        />
      </div>

      <div className="tabs">
        <button className={`tab ${tab === 'all' ? 'active' : ''}`} onClick={() => setTab('all')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
            <rect x="3" y="3" width="7" height="7" /><rect x="14" y="3" width="7" height="7" />
            <rect x="3" y="14" width="7" height="7" /><rect x="14" y="14" width="7" height="7" />
          </svg>
          All
        </button>
        <button className={`tab ${tab === 'text' ? 'active' : ''}`} onClick={() => setTab('text')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
            <path d="M4 7V4h16v3" /><path d="M9 20h6" /><path d="M12 4v16" />
          </svg>
          Text
        </button>
        <button className={`tab ${tab === 'image' ? 'active' : ''}`} onClick={() => setTab('image')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
            <rect x="3" y="3" width="18" height="18" rx="2" />
            <circle cx="8.5" cy="8.5" r="1.5" />
            <path d="m21 15-5-5L5 21" />
          </svg>
          Images
        </button>
        <button className={`tab ${tab === 'favorites' ? 'active' : ''}`} onClick={() => setTab('favorites')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
            <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
          </svg>
          Favorites
        </button>
        <button className={`tab ${tab === 'snippets' ? 'active' : ''}`} onClick={() => setTab('snippets')}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
            <polyline points="14 2 14 8 20 8" />
            <line x1="16" y1="13" x2="8" y2="13" />
            <line x1="16" y1="17" x2="8" y2="17" />
          </svg>
          Snippets
        </button>
      </div>

      <div className="content">
        {tab === 'snippets' ? (
          <>
            <button className="add-snippet-btn" onClick={() => setShowSnippetForm(!showSnippetForm)}>
              {showSnippetForm ? 'Cancel' : '+ New Snippet'}
            </button>
            {showSnippetForm && (
              <div className="snippet-form">
                <input
                  type="text"
                  placeholder="Title"
                  value={snippetTitle}
                  onChange={e => setSnippetTitle(e.target.value)}
                />
                <textarea
                  placeholder="Content"
                  value={snippetContent}
                  onChange={e => setSnippetContent(e.target.value)}
                  rows={3}
                />
                <button className="save-snippet-btn" onClick={addSnippet}>Save</button>
              </div>
            )}
            <div className="snippet-list">
              {snippets.map(s => (
                <div key={s.id} className="snippet-card" onClick={() => copyText(s.content)}>
                  <div className="snippet-title">{s.title}</div>
                  <div className="snippet-text">{s.content}</div>
                  <button className="delete-btn" onClick={e => { e.stopPropagation(); deleteSnippet(s.id) }}>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
                      <path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                    </svg>
                  </button>
                </div>
              ))}
            </div>
          </>
        ) : (
          <div className="item-list">
            {items.map(item => (
              <div key={item.id} className={`item ${item.content_type === 'image' ? 'item-image' : ''}`} onClick={() => copyToClipboard(item)}>
                {item.content_type === 'image' && item.thumbnail ? (
                  <div className="item-image-wrapper">
                    <img src={item.thumbnail} alt="clipboard image" className="item-image-preview" />
                  </div>
                ) : (
                  <div className="item-content">{item.content}</div>
                )}
                <div className="item-meta">
                  <span className="item-time">{formatTime(item.created_at)}</span>
                  <div className="item-actions">
                    <button className={`fav-btn ${item.is_favorite ? 'active' : ''}`} onClick={e => { e.stopPropagation(); toggleFavorite(item.id) }}>
                      <svg viewBox="0 0 24 24" fill={item.is_favorite ? 'currentColor' : 'none'} stroke="currentColor" strokeWidth="2" width="14" height="14">
                        <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
                      </svg>
                    </button>
                    <button className="delete-btn" onClick={e => { e.stopPropagation(); deleteItem(item.id) }}>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
                        <path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                      </svg>
                    </button>
                  </div>
                </div>
              </div>
            ))}
            {items.length === 0 && <div className="empty">No items yet. Copy something to get started.</div>}
          </div>
        )}
      </div>
    </div>
  )
}
