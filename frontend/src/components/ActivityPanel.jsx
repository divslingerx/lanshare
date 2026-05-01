export default function ActivityPanel({ transfers }) {
  const active    = transfers.filter(t => !t.done);
  const completed = transfers.filter(t =>  t.done);

  return (
    <aside style={{ width: 276, minWidth: 276, background: 'var(--surface)', borderLeft: '1px solid var(--border)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <div style={{ padding: '18px 18px 14px', borderBottom: '1px solid var(--border-2)' }}>
        <div style={{ fontFamily: 'Syne, sans-serif', fontSize: 14, fontWeight: 700 }}>Activity</div>
        <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10, color: 'var(--text-3)', marginTop: 1 }}>
          {active.length} active · {completed.length} completed
        </div>
      </div>

      <div style={{ flex: 1, overflowY: 'auto', padding: '12px 10px', display: 'flex', flexDirection: 'column', gap: 7 }}>
        {active.length > 0 && (
          <>
            <div className="sec-label" style={{ padding: '0 2px' }}>Active</div>
            {active.map(t => (
              <div key={t.id} style={{ background: 'var(--surface-2)', border: '1px solid var(--border-2)', borderRadius: 'var(--r-sm)', padding: '11px 12px' }}>
                <div style={{ display: 'flex', alignItems: 'flex-start', gap: 9, marginBottom: 9 }}>
                  <div style={{ width: 26, height: 26, borderRadius: 6, display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0, background: 'var(--accent-bg)', color: 'var(--accent)' }}>
                    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
                      {t.direction === 'download'
                        ? <><polyline points="8 17 12 21 16 17"/><line x1="12" y1="12" x2="12" y2="21"/><path d="M20.88 18.09A5 5 0 0018 9h-1.26A8 8 0 103 16.29"/></>
                        : <><polyline points="16 17 12 13 8 17"/><line x1="12" y1="13" x2="12" y2="21"/><path d="M20.88 18.09A5 5 0 0018 9h-1.26A8 8 0 103 16.29"/></>}
                    </svg>
                  </div>
                  <div>
                    <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 11, lineHeight: 1.4, wordBreak: 'break-all' }}>{t.filename}</div>
                    <div style={{ fontSize: 10.5, color: 'var(--text-3)', marginTop: 1 }}>{t.direction === 'download' ? 'from' : 'to'} {t.peer}</div>
                  </div>
                </div>
                <div style={{ height: 3, background: 'var(--surface-3)', borderRadius: 2, overflow: 'hidden', marginBottom: 6 }}>
                  <div style={{ height: '100%', borderRadius: 2, background: 'var(--accent)', width: `${t.pct || 0}%`, transition: 'width 0.3s' }} />
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <span style={{ fontFamily: 'DM Mono, monospace', fontSize: 10, color: 'var(--text-3)' }}>{t.speed || '—'}</span>
                  <span style={{ fontFamily: 'DM Mono, monospace', fontSize: 10, color: 'var(--text-2)', fontWeight: 500 }}>{t.pct || 0}%</span>
                </div>
              </div>
            ))}
          </>
        )}

        {completed.length > 0 && (
          <>
            <div className="sec-label" style={{ padding: '6px 2px 0' }}>Completed</div>
            {completed.slice(0, 10).map(t => (
              <div key={t.id} style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '9px 11px', background: 'var(--surface-2)', border: '1px solid var(--border-2)', borderRadius: 'var(--r-sm)' }}>
                <div style={{ width: 22, height: 22, borderRadius: '50%', background: 'var(--accent-bg)', color: 'var(--accent)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3"><polyline points="20 6 9 17 4 12"/></svg>
                </div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 11, color: 'var(--text-2)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{t.filename}</div>
                  <div style={{ fontSize: 10.5, color: 'var(--text-3)' }}>{t.direction === 'download' ? 'from' : 'to'} {t.peer}</div>
                </div>
              </div>
            ))}
          </>
        )}

        {transfers.length === 0 && (
          <p style={{ padding: '12px 4px', fontSize: 12, color: 'var(--text-3)' }}>No activity yet.</p>
        )}
      </div>

      <div style={{ padding: '10px 18px', borderTop: '1px solid var(--border-2)', display: 'flex', alignItems: 'center', gap: 8 }}>
        <div style={{ width: 6, height: 6, borderRadius: '50%', background: 'var(--accent)', animation: 'heartbeat 2.4s ease-in-out infinite' }} />
        <span style={{ fontFamily: 'DM Mono, monospace', fontSize: 10.5, color: 'var(--text-3)' }}>Listening</span>
      </div>
    </aside>
  );
}
