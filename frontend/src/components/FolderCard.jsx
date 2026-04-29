function basename(p) {
  return p.replace(/\\/g, '/').split('/').filter(Boolean).pop() || p;
}

export default function FolderCard({ folder, onRemove, onSetMode }) {
  const isShared = folder.mode === 'shared';
  const modeColor = isShared ? 'var(--shared)' : 'var(--watch)';
  const modeBg    = isShared ? 'var(--shared-bg)' : 'var(--watch-bg)';

  return (
    <div style={{
      display: 'flex', alignItems: 'stretch',
      background: 'var(--surface)', border: '1px solid var(--border)',
      borderRadius: 'var(--r)', overflow: 'hidden',
      transition: 'border-color 0.18s, box-shadow 0.18s',
      animation: 'slideIn 0.18s ease-out',
    }}>
      <style>{`@keyframes slideIn { from { opacity:0; transform:translateY(6px); } to { opacity:1; transform:translateY(0); } }`}</style>
      <div style={{ width: 3, flexShrink: 0, background: modeColor, transition: 'background 0.3s' }} />

      <div style={{ flex: 1, display: 'flex', alignItems: 'center', gap: 14, padding: '13px 16px', minWidth: 0 }}>
        <div style={{ width: 36, height: 36, borderRadius: 8, background: modeBg, display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
          <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke={modeColor} strokeWidth="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>
        </div>

        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: 13, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {basename(folder.path)}
          </div>
          <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10.5, color: 'var(--text-3)', marginTop: 2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={folder.path}>
            {folder.path}
          </div>
        </div>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', paddingRight: 12, gap: 6 }}>
        <div style={{ display: 'flex', background: 'var(--surface-2)', border: '1px solid var(--border-2)', borderRadius: 20, padding: 3, gap: 2 }}>
          {['watch', 'shared'].map(m => (
            <button
              key={m}
              onClick={() => onSetMode(folder.id, m)}
              style={{
                padding: '4px 11px', borderRadius: 18, fontSize: 11.5, fontWeight: 500,
                cursor: 'pointer', border: folder.mode === m ? `1px solid ${m === 'watch' ? 'rgba(50,208,122,0.18)' : 'rgba(245,166,35,0.18)'}` : 'none',
                background: folder.mode === m ? (m === 'watch' ? 'var(--watch-bg)' : 'var(--shared-bg)') : 'transparent',
                color: folder.mode === m ? (m === 'watch' ? 'var(--watch)' : 'var(--shared)') : 'var(--text-3)',
                fontFamily: 'DM Sans, sans-serif', transition: 'all 0.18s', whiteSpace: 'nowrap',
              }}
            >
              {m.charAt(0).toUpperCase() + m.slice(1)}
            </button>
          ))}
        </div>

        <button
          onClick={() => onRemove(folder.id)}
          title="Remove folder"
          style={{ width: 30, height: 30, borderRadius: 'var(--r-sm)', background: 'transparent', border: 'none', color: 'var(--text-3)', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', transition: 'all 0.12s' }}
          onMouseEnter={e => { e.currentTarget.style.background = 'rgba(239,88,88,0.12)'; e.currentTarget.style.color = 'var(--danger)'; }}
          onMouseLeave={e => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--text-3)'; }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>
    </div>
  );
}
