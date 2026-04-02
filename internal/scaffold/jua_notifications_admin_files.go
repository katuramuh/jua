package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaNotificationsAdminFiles(root string, opts Options) error {
	if !opts.ShouldIncludeAdmin() {
		return nil
	}

	adminRoot := filepath.Join(root, "apps", "admin")

	files := map[string]string{
		filepath.Join(adminRoot, "app", "(dashboard)", "realtime", "page.tsx"):      adminRealtimePageTSX(),
		filepath.Join(adminRoot, "app", "(dashboard)", "notifications", "page.tsx"): adminNotificationsPageTSX(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{PROJECT_NAME}}", opts.ProjectName)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func adminRealtimePageTSX() string {
	return `'use client'

import { useEffect, useState } from 'react'
import { Radio, Users, Activity } from 'lucide-react'

interface HubStats {
  total_connections: number
  connections_by_tenant: Record<string, number>
  sse_connections: number
  ws_connections: number
}

interface RecentEvent {
  id: string
  type: string
  tenantId: string
  createdAt: string
}

function StatsCard({ label, value, sub }: { label: string; value: number; sub?: string }) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)] p-5">
      <p className="text-xs text-[var(--text-muted)] uppercase tracking-wide">{label}</p>
      <p className="mt-2 text-3xl font-bold text-[var(--text-primary)]">{value}</p>
      {sub && <p className="mt-1 text-xs text-[var(--text-secondary)]">{sub}</p>}
    </div>
  )
}

export default function RealtimePage() {
  const [stats, setStats] = useState<HubStats | null>(null)
  const [events, setEvents] = useState<RecentEvent[]>([])
  const [connected, setConnected] = useState(false)

  // Poll stats every 5 seconds
  useEffect(() => {
    const load = async () => {
      try {
        const token = localStorage.getItem('jua_token') || ''
        const res = await fetch('/api/realtime/stats', {
          headers: { Authorization: ` + "`" + `Bearer ${token}` + "`" + ` },
        })
        if (res.ok) setStats(await res.json())
      } catch {}
    }
    load()
    const interval = setInterval(load, 5000)
    return () => clearInterval(interval)
  }, [])

  // Live event stream
  useEffect(() => {
    const token = localStorage.getItem('jua_token') || ''
    if (!token) return
    const es = new EventSource(` + "`" + `/api/realtime/stream?token=${token}` + "`" + `)
    es.onopen = () => setConnected(true)
    es.onerror = () => setConnected(false)
    es.onmessage = (e) => {
      try {
        const event = JSON.parse(e.data)
        setEvents((prev) => [event, ...prev].slice(0, 50))
      } catch {}
    }
    return () => es.close()
  }, [])

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Radio className="h-6 w-6 text-[var(--accent)]" />
          <h1 className="text-xl font-semibold text-[var(--text-primary)]">Real-time Engine</h1>
        </div>
        <div className="flex items-center gap-1.5 text-xs text-[var(--text-secondary)]">
          <span className={` + "`" + `h-2 w-2 rounded-full ${connected ? 'bg-green-500' : 'bg-muted-foreground/40 animate-pulse'}` + "`" + `} />
          {connected ? 'Live' : 'Connecting…'}
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatsCard label="Total Connections" value={stats?.total_connections ?? 0} />
        <StatsCard label="SSE Connections" value={stats?.sse_connections ?? 0} sub="Server-Sent Events" />
        <StatsCard label="WebSocket Connections" value={stats?.ws_connections ?? 0} sub="Bidirectional" />
        <StatsCard
          label="Active Tenants"
          value={Object.keys(stats?.connections_by_tenant ?? {}).length}
        />
      </div>

      {/* Connections by tenant */}
      {stats && Object.keys(stats.connections_by_tenant).length > 0 && (
        <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)] p-5">
          <div className="flex items-center gap-2 mb-4">
            <Users className="h-4 w-4 text-[var(--text-secondary)]" />
            <h2 className="text-sm font-semibold text-[var(--text-primary)]">Connections by Tenant</h2>
          </div>
          <div className="space-y-2">
            {Object.entries(stats.connections_by_tenant).map(([tenantId, count]) => (
              <div key={tenantId} className="flex items-center justify-between text-sm">
                <span className="text-[var(--text-secondary)] font-mono text-xs truncate max-w-xs">
                  {tenantId}
                </span>
                <span className="ml-2 shrink-0 text-[var(--text-primary)] font-medium">{count}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Recent events log */}
      <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)] overflow-hidden">
        <div className="flex items-center gap-2 px-5 py-3.5 border-b border-[var(--border)]">
          <Activity className="h-4 w-4 text-[var(--text-secondary)]" />
          <h2 className="text-sm font-semibold text-[var(--text-primary)]">Live Event Log</h2>
          <span className="ml-auto text-xs text-[var(--text-muted)]">last 50 events</span>
        </div>
        <div className="divide-y divide-[var(--border)] max-h-96 overflow-y-auto">
          {events.length === 0 ? (
            <p className="px-5 py-8 text-center text-sm text-[var(--text-muted)]">
              No events yet — waiting for activity…
            </p>
          ) : (
            events.map((ev) => (
              <div key={ev.id} className="px-5 py-2.5 flex items-center gap-3 text-xs">
                <span className="shrink-0 font-mono text-[var(--accent)]">{ev.type}</span>
                <span className="text-[var(--text-muted)] truncate">{ev.tenantId}</span>
                <span className="ml-auto shrink-0 text-[var(--text-muted)]">
                  {new Date(ev.createdAt).toLocaleTimeString()}
                </span>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  )
}
`
}

func adminNotificationsPageTSX() string {
	return `'use client'

import { useEffect, useState } from 'react'
import { Bell, BarChart2, AlertCircle, RefreshCw } from 'lucide-react'

interface Stats {
  total_today: number
  by_channel: Array<{ channel: string; sent: number; failed: number }>
}

interface LogEntry {
  id: string
  notification_id: string
  channel: string
  status: string
  error: string
  sent_at: string
}

function authHeaders() {
  const token = localStorage.getItem('jua_token') || ''
  return { Authorization: ` + "`" + `Bearer ${token}` + "`" + `, 'Content-Type': 'application/json' }
}

export default function NotificationsPage() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [broadcastForm, setBroadcastForm] = useState({
    tenant_id: '', title: '', body: '', type: 'operational',
  })
  const [broadcasting, setBroadcasting] = useState(false)
  const [broadcastMsg, setBroadcastMsg] = useState('')

  const loadData = async () => {
    const [sRes, lRes] = await Promise.all([
      fetch('/api/admin/notifications/stats', { headers: authHeaders() }),
      fetch('/api/admin/notifications/logs?limit=50', { headers: authHeaders() }),
    ])
    if (sRes.ok) setStats((await sRes.json()).data)
    if (lRes.ok) setLogs((await lRes.json()).data ?? [])
  }

  useEffect(() => { loadData() }, [])

  const handleBroadcast = async () => {
    setBroadcasting(true)
    setBroadcastMsg('')
    try {
      const res = await fetch('/api/admin/notifications/broadcast', {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify(broadcastForm),
      })
      const json = await res.json()
      setBroadcastMsg(res.ok ? ` + "`" + `Sent to ${json.data?.recipients ?? 0} users` + "`" + ` : json.error?.message ?? 'Error')
      if (res.ok) loadData()
    } catch {
      setBroadcastMsg('Network error')
    }
    setBroadcasting(false)
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Bell className="h-6 w-6 text-[var(--accent)]" />
        <h1 className="text-xl font-semibold text-[var(--text-primary)]">Notifications Centre</h1>
        <button
          onClick={loadData}
          className="ml-auto p-2 rounded-lg hover:bg-[var(--bg-hover)] transition-colors"
          title="Refresh"
        >
          <RefreshCw className="h-4 w-4 text-[var(--text-secondary)]" />
        </button>
      </div>

      {/* Stats */}
      {stats && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)] p-5 col-span-2 lg:col-span-1">
            <p className="text-xs text-[var(--text-muted)] uppercase tracking-wide">Sent Today</p>
            <p className="mt-2 text-3xl font-bold text-[var(--text-primary)]">{stats.total_today}</p>
          </div>
          {stats.by_channel.map((ch) => (
            <div key={ch.channel} className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)] p-5">
              <p className="text-xs text-[var(--text-muted)] uppercase tracking-wide">{ch.channel}</p>
              <p className="mt-2 text-2xl font-bold text-[var(--text-primary)]">{ch.sent}</p>
              {ch.failed > 0 && (
                <p className="mt-1 text-xs text-[var(--danger)]">{ch.failed} failed</p>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Broadcast form */}
      <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)] p-5 space-y-4">
        <div className="flex items-center gap-2">
          <BarChart2 className="h-4 w-4 text-[var(--text-secondary)]" />
          <h2 className="text-sm font-semibold text-[var(--text-primary)]">Broadcast Message</h2>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="text-xs text-[var(--text-secondary)]">Tenant ID</label>
            <input
              value={broadcastForm.tenant_id}
              onChange={(e) => setBroadcastForm((f) => ({ ...f, tenant_id: e.target.value }))}
              placeholder="uuid"
              className="mt-1 w-full px-3 py-2 rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border)] text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)]"
            />
          </div>
          <div>
            <label className="text-xs text-[var(--text-secondary)]">Type</label>
            <select
              value={broadcastForm.type}
              onChange={(e) => setBroadcastForm((f) => ({ ...f, type: e.target.value }))}
              className="mt-1 w-full px-3 py-2 rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border)] text-sm text-[var(--text-primary)]"
            >
              <option value="operational">Operational</option>
              <option value="marketing">Marketing</option>
              <option value="transactional">Transactional</option>
            </select>
          </div>
          <div>
            <label className="text-xs text-[var(--text-secondary)]">Title</label>
            <input
              value={broadcastForm.title}
              onChange={(e) => setBroadcastForm((f) => ({ ...f, title: e.target.value }))}
              placeholder="Notification title"
              className="mt-1 w-full px-3 py-2 rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border)] text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)]"
            />
          </div>
          <div>
            <label className="text-xs text-[var(--text-secondary)]">Body</label>
            <input
              value={broadcastForm.body}
              onChange={(e) => setBroadcastForm((f) => ({ ...f, body: e.target.value }))}
              placeholder="Notification message"
              className="mt-1 w-full px-3 py-2 rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border)] text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)]"
            />
          </div>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={handleBroadcast}
            disabled={broadcasting || !broadcastForm.tenant_id || !broadcastForm.title}
            className="px-5 py-2 rounded-lg bg-[var(--accent)] hover:bg-[var(--accent-hover)] text-white text-sm font-medium transition-colors disabled:opacity-60"
          >
            {broadcasting ? 'Sending…' : 'Send Broadcast'}
          </button>
          {broadcastMsg && (
            <span className="text-sm text-[var(--text-secondary)]">{broadcastMsg}</span>
          )}
        </div>
      </div>

      {/* Log table */}
      <div className="rounded-xl border border-[var(--border)] bg-[var(--bg-elevated)] overflow-hidden">
        <div className="flex items-center gap-2 px-5 py-3.5 border-b border-[var(--border)]">
          <AlertCircle className="h-4 w-4 text-[var(--text-secondary)]" />
          <h2 className="text-sm font-semibold text-[var(--text-primary)]">Delivery Log</h2>
          <span className="ml-auto text-xs text-[var(--text-muted)]">last 50</span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead className="border-b border-[var(--border)]">
              <tr className="text-left text-[var(--text-muted)]">
                <th className="px-4 py-2.5 font-medium">Channel</th>
                <th className="px-4 py-2.5 font-medium">Status</th>
                <th className="px-4 py-2.5 font-medium">Notification ID</th>
                <th className="px-4 py-2.5 font-medium">Error</th>
                <th className="px-4 py-2.5 font-medium">Sent At</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--border)]">
              {logs.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-4 py-8 text-center text-[var(--text-muted)]">
                    No delivery logs yet
                  </td>
                </tr>
              ) : (
                logs.map((log) => (
                  <tr key={log.id} className="hover:bg-[var(--bg-hover)] transition-colors">
                    <td className="px-4 py-2.5 font-medium text-[var(--text-primary)] uppercase">
                      {log.channel}
                    </td>
                    <td className="px-4 py-2.5">
                      <span
                        className={` + "`" + `px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase ${
                          log.status === 'sent' ? 'bg-green-500/10 text-green-400'
                          : log.status === 'failed' ? 'bg-red-500/10 text-red-400'
                          : 'bg-yellow-500/10 text-yellow-400'
                        }` + "`" + `}
                      >
                        {log.status}
                      </span>
                    </td>
                    <td className="px-4 py-2.5 font-mono text-[var(--text-muted)] truncate max-w-[120px]">
                      {log.notification_id}
                    </td>
                    <td className="px-4 py-2.5 text-[var(--danger)] truncate max-w-[200px]">
                      {log.error || '—'}
                    </td>
                    <td className="px-4 py-2.5 text-[var(--text-muted)]">
                      {new Date(log.sent_at).toLocaleString()}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
`
}
