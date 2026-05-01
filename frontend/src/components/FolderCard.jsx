function basename(p) {
  return p.replace(/\\/g, '/').split('/').filter(Boolean).pop() || p;
}

export default function FolderCard({ folder, onRemove, onToggleSharing }) {
  const sharing = !folder.disabled;

  return (
    <div style={{
      display: 'flex', alignItems: 'stretch',
      background: 'var(--surface)', border: '1px solid var(--border)',
      borderRadius: 'var(--r)', overflow: 'hidden',
      opacity: sharing ? 1 : 0.6,
      transition: 'opacity 0.2s, border-color 0.18s',
      animation: 'slideIn 0.18s ease-out',
    }}>
      <style>{`@keyframes slideIn { from { opacity:0; transform:translateY(6px); } to { opacity:1; transform:translateY(0); } }`}</style>
      <div style={{ width: 3, flexShrink: 0, background: sharing ? 'var(--accent)' : 'var(--border-2)', transition: 'background 0.2s' }} />

      <div style={{ flex: 1, display: 'flex', alignItems: 'center', gap: 14, padding: '13px 16px', minWidth: 0 }}>
        <div style={{ width: 36, height: 36, borderRadius: 8, background: sharing ? 'rgba(50,208,122,0.12)' : 'var(--surface-2)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0, transition: 'background 0.2s' }}>
          <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke={sharing ? 'var(--accent)' : 'var(--text-3)'} strokeWidth="2" style={{ transition: 'stroke 0.2s' }}><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>
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

      <div style={{ display: 'flex', alignItems: 'center', paddingRight: 12, gap: 10 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
          <span style={{ fontSize: 11, color: sharing ? 'var(--accent)' : 'var(--text-3)', fontWeight: 500, transition: 'color 0.2s' }}>
            {sharing ? 'Sharing' : 'Paused'}
          </span>
          <button
            onClick={() => onToggleSharing(folder.id, !sharing)}
            title={sharing ? 'Pause sharing' : 'Resume sharing'}
            style={{
              width: 38, height: 22, borderRadius: 11, padding: 0,
              background: sharing ? 'var(--accent)' : 'var(--surface-2)',
              border: `1.5px solid ${sharing ? 'var(--accent)' : 'var(--border-2)'}`,
              cursor: 'pointer', position: 'relative', transition: 'all 0.2s', flexShrink: 0,
            }}
          >
            <span style={{
              position: 'absolute', top: 2,
              left: sharing ? 18 : 2,
              width: 14, height: 14, borderRadius: '50%',
              background: sharing ? '#fff' : 'var(--text-3)',
              transition: 'left 0.2s, background 0.2s',
            }} />
          </button>
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
