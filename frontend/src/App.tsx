import { useState, useEffect, useRef, useCallback } from 'react'
import './App.css'

interface HistoryItem {
  id: number
  content_type: string
  content: string
  is_favorite: boolean
  created_at: string
  thumbnail?: string
  category?: string
  eval_result?: string
}

interface SearchResultItem extends HistoryItem {
  score?: number
  match_ranges?: [number, number][]
  match_type?: string
}

interface Snippet {
  id: number
  title: string
  content: string
  created_at: string
}

type Tab = 'all' | 'text' | 'image' | 'favorites' | 'snippets'

const CATEGORY_ICONS: Record<string, string> = {
  link: '🔗',
  email: '✉',
  phone: '📞',
  json: '{ }',
  code: '</>',
  math: '=',
  template: '{{ }}',
}

export default function App() {
  const [items, setItems] = useState<HistoryItem[]>([])
  const [snippets, setSnippets] = useState<Snippet[]>([])
  const [tab, setTab] = useState<Tab>('all')
  const [query, setQuery] = useState('')
  const [snippetTitle, setSnippetTitle] = useState('')
  const [snippetContent, setSnippetContent] = useState('')
  const [showSnippetForm, setShowSnippetForm] = useState(false)
  const [isSearching, setIsSearching] = useState(false)
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [formatMenu, setFormatMenu] = useState<number | null>(null)
  const [snippetVars, setSnippetVars] = useState<Record<string, string>>({})
  const [zapMode, setZapMode] = useState(false)
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
    drag.winX = window.screenX
    drag.winY = window.screenY
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
    setIsSearching(!!q.trim())
    if (!q.trim()) {
      fetchHistory(tab === 'favorites' ? 'favorites' : tab === 'text' ? 'text' : tab === 'image' ? 'image' : '')
      return
    }
    try {
      const res = await fetch(`/api/search?mode=fuzzy&q=${encodeURIComponent(q)}&limit=50`)
      const data = await res.json()
      setItems(data)
      setZapMode(true)
    } catch {}
  }, [tab, fetchHistory])

  const handleSearchKeyDown = useCallback(async (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && zapMode && items.length > 0) {
      const first = items[0]
      await copyToClipboard(first)
      ;(window as any).__toggle?.()
    }
    if (e.key === 'Escape') {
      ;(window as any).__toggle?.()
    }
  }, [zapMode, items])

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

  const copyToClipboard = useCallback(async (item: HistoryItem, opts?: { plain?: boolean }) => {
    try {
      const body: Record<string, any> = {}
      if (item.content_type === 'image' && item.thumbnail) {
        body.image_path = item.content
      } else if (item.content) {
        body.text = item.content
        if (opts?.plain) body.plain = true
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

  const copyWithFormat = useCallback(async (item: HistoryItem, action: string) => {
    try {
      const res = await fetch(`/api/format?action=${action}&text=${encodeURIComponent(item.content)}`)
      const data = await res.json()
      await fetch(`/api/copy`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: data.result }),
      })
    } catch {}
    setFormatMenu(null)
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

  const pasteSnippet = useCallback(async (content: string) => {
    const vars: Record<string, string> = {}
    const matches = content.match(/\{\{(\w+)\}\}/g)
    if (matches) {
      for (const m of matches) {
        const key = m.slice(2, -2)
        const val = snippetVars[key] || ''
        vars[key] = val
      }
    }
    try {
      await fetch(`/api/copy`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: content, vars }),
      })
    } catch {}
  }, [snippetVars])

  const pasteAllSelected = useCallback(async () => {
    const selected = items.filter(i => selectedIds.has(i.id))
    if (selected.length === 0) return
    try {
      await fetch(`/api/paste`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          items: selected.map(i => ({ text: i.content })),
          delay: 200,
        }),
      })
    } catch {}
    setSelectedIds(new Set())
  }, [items, selectedIds])

  const toggleSelect = useCallback((id: number) => {
    setSelectedIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const formatTime = (iso: string) => {
    const d = new Date(iso)
    const now = new Date()
    const diff = now.getTime() - d.getTime()
    if (diff < 60000) return 'just now'
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`
    return d.toLocaleDateString()
  }

  const highlightText = (text: string, ranges?: [number, number][]) => {
    if (!ranges || ranges.length === 0 || !isSearching) {
      return <span className="item-content-text">{text}</span>
    }
    const segments: { start: number; end: number; highlight: boolean }[] = []
    let lastEnd = 0
    for (const [s, e] of ranges) {
      if (s > lastEnd) segments.push({ start: lastEnd, end: s, highlight: false })
      segments.push({ start: s, end: e, highlight: true })
      lastEnd = e
    }
    if (lastEnd < text.length) segments.push({ start: lastEnd, end: text.length, highlight: false })
    return (
      <span className="item-content-text">
        {segments.map((seg, i) =>
          seg.highlight ? (
            <mark key={i} className="highlight">{text.slice(seg.start, seg.end)}</mark>
          ) : (
            <span key={i}>{text.slice(seg.start, seg.end)}</span>
          )
        )}
      </span>
    )
  }

  const formatActions = [
    { id: 'json', label: 'Format JSON' },
    { id: 'json-minify', label: 'Minify JSON' },
    { id: 'base64-encode', label: 'Base64 Encode' },
    { id: 'base64-decode', label: 'Base64 Decode' },
    { id: 'url-encode', label: 'URL Encode' },
    { id: 'url-decode', label: 'URL Decode' },
    { id: 'upper', label: 'UPPERCASE' },
    { id: 'lower', label: 'lowercase' },
    { id: 'trim', label: 'Trim Spaces' },
    { id: 'lines-sort', label: 'Reverse Lines' },
    { id: 'lines-uniq', label: 'Unique Lines' },
  ]

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
          placeholder={zapMode ? "Search... (Enter = paste top result)" : "Search clipboard..."}
          value={query}
          onChange={e => handleSearch(e.target.value)}
          onKeyDown={handleSearchKeyDown}
        />
        {zapMode && <span className="zap-badge">Zap</span>}
      </div>

      <div className="tabs">
        <button className={`tab ${tab === 'all' ? 'active' : ''}`} onClick={() => setTab('all')}>All</button>
        <button className={`tab ${tab === 'text' ? 'active' : ''}`} onClick={() => setTab('text')}>Text</button>
        <button className={`tab ${tab === 'image' ? 'active' : ''}`} onClick={() => setTab('image')}>Images</button>
        <button className={`tab ${tab === 'favorites' ? 'active' : ''}`} onClick={() => setTab('favorites')}>★ Favorites</button>
        <button className={`tab ${tab === 'snippets' ? 'active' : ''}`} onClick={() => setTab('snippets')}>Snippets</button>
      </div>

      <div className="content">
        {tab === 'snippets' ? (
          <>
            <div className="snippet-vars">
              <input
                type="text"
                placeholder="{{name}} value"
                value={snippetVars.name || ''}
                onChange={e => setSnippetVars(prev => ({ ...prev, name: e.target.value }))}
              />
              <input
                type="text"
                placeholder="{{date}} value"
                value={snippetVars.date || ''}
                onChange={e => setSnippetVars(prev => ({ ...prev, date: e.target.value }))}
              />
            </div>
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
                  placeholder="Content (use {{name}}, {{date}}, {{time}})"
                  value={snippetContent}
                  onChange={e => setSnippetContent(e.target.value)}
                  rows={3}
                />
                <button className="save-snippet-btn" onClick={addSnippet}>Save</button>
              </div>
            )}
            <div className="snippet-list">
              {snippets.map(s => {
                const hasVars = s.content.includes('{{')
                return (
                  <div key={s.id} className="snippet-card" onClick={() => pasteSnippet(s.content)}>
                    <div className="snippet-title">{s.title}</div>
                    <div className="snippet-text">{s.content}</div>
                    {hasVars && <span className="snippet-vars-badge">Variables</span>}
                    <button className="delete-btn" onClick={e => { e.stopPropagation(); deleteSnippet(s.id) }}>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
                        <path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                      </svg>
                    </button>
                  </div>
                )
              })}
            </div>
          </>
        ) : (
          <>
            {selectedIds.size > 0 && (
              <div className="queue-bar">
                <span className="queue-count">{selectedIds.size} selected</span>
                <button className="paste-all-btn" onClick={pasteAllSelected}>
                  Paste All ({selectedIds.size})
                </button>
                <button className="queue-clear" onClick={() => setSelectedIds(new Set())}>Clear</button>
              </div>
            )}
            <div className="item-list">
              {items.map(item => (
                <div key={item.id} className={`item ${item.content_type === 'image' ? 'item-image' : ''}`}>
                  <div className="item-main" onClick={() => copyToClipboard(item)}>
                    {item.content_type === 'image' && item.thumbnail ? (
                      <div className="item-image-wrapper">
                        <img src={item.thumbnail} alt="clipboard image" className="item-image-preview" />
                      </div>
                    ) : (
                      <div className="item-content">
                        {item.eval_result && <span className="math-badge">= {item.eval_result}</span>}
                        {(item as SearchResultItem).match_ranges
                          ? highlightText(item.content, (item as SearchResultItem).match_ranges)
                          : <span className="item-content-text">{item.content}</span>
                        }
                      </div>
                    )}
                    <div className="item-meta">
                      <div className="item-meta-left">
                        {item.category && (
                          <span className={`category-badge category-${item.category}`}>
                            {CATEGORY_ICONS[item.category] || item.category}
                          </span>
                        )}
                        <span className="item-time">{formatTime(item.created_at)}</span>
                      </div>
                      <div className="item-actions">
                        <button
                          className="select-btn"
                          onClick={e => { e.stopPropagation(); toggleSelect(item.id) }}
                        >
                          {selectedIds.has(item.id) ? '✓' : '□'}
                        </button>
                        <button
                          className="plain-btn"
                          onClick={e => { e.stopPropagation(); copyToClipboard(item, { plain: true }) }}
                          title="Paste as plain text"
                        >
                          Tt
                        </button>
                        <button
                          className="format-btn"
                          onClick={e => { e.stopPropagation(); setFormatMenu(formatMenu === item.id ? null : item.id) }}
                          title="Format tools"
                        >
                          ▼
                        </button>
                        <button className={`fav-btn ${item.is_favorite ? 'active' : ''}`} onClick={e => { e.stopPropagation(); toggleFavorite(item.id) }}>
                          {item.is_favorite ? '★' : '☆'}
                        </button>
                        <button className="delete-btn" onClick={e => { e.stopPropagation(); deleteItem(item.id) }}>
                          ✕
                        </button>
                      </div>
                    </div>
                    {formatMenu === item.id && (
                      <div className="format-menu">
                        {formatActions.map(a => (
                          <button
                            key={a.id}
                            className="format-menu-item"
                            onClick={e => { e.stopPropagation(); copyWithFormat(item, a.id) }}
                          >
                            {a.label}
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              ))}
              {items.length === 0 && <div className="empty">{isSearching ? 'No matches found' : 'No items yet. Copy something to get started.'}</div>}
            </div>
          </>
        )}
      </div>
    </div>
  )
}
