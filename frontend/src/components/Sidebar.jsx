const pulseStyle = (color) => ({
  width: 7, height: 7, borderRadius: '50%',
  background: color || 'var(--watch)',
  flexShrink: 0,
  animation: 'heartbeat 2.4s ease-in-out infinite',
});

const styles = `
@keyframes heartbeat {
  0%,100% { box-shadow: 0 0 0 0 rgba(50,208,122,0.5); }
  40%      { box-shadow: 0 0 0 5px rgba(50,208,122,0); }
}`;

export default function Sidebar({ activeView, onNavChange, peers, config, folderCount }) {
  return (
    <aside style={{ width: 238, minWidth: 238, background: 'var(--surface)', borderRight: '1px solid var(--border)', display: 'flex', flexDirection: 'column' }}>
      <style>{styles}</style>

      <div style={{ padding: '18px 18px 14px', borderBottom: '1px solid var(--border-2)', display: 'flex', alignItems: 'center', gap: 10 }}>
        <div style={{ width: 30, height: 30, borderRadius: 8, background: 'var(--accent)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.5"><circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/></svg>
        </div>
        <span style={{ fontFamily: 'Syne, sans-serif', fontSize: 16, fontWeight: 700, letterSpacing: '-0.4px' }}>filehub</span>
        <span style={{ marginLeft: 'auto', fontFamily: 'DM Mono, monospace', fontSize: 9, color: 'var(--text-3)', letterSpacing: '0.8px' }}>v0.1</span>
      </div>

      <nav style={{ padding: '10px 8px', borderBottom: '1px solid var(--border-2)' }}>
        {[
          { id: 'folders', label: 'My Folders', badge: folderCount },
          { id: 'network', label: 'Network',    badge: peers.length },
          { id: 'settings', label: 'Settings',  badge: null },
        ].map(({ id, label, badge }) => (
          <div
            key={id}
            onClick={() => onNavChange(id)}
            style={{
              display: 'flex', alignItems: 'center', gap: 9,
              padding: '7px 10px', borderRadius: 'var(--r-sm)',
              cursor: 'pointer', fontSize: 13, fontWeight: 500,
              background: activeView === id ? 'var(--accent-bg)' : 'transparent',
              color: activeView === id ? 'var(--accent)' : 'var(--text-2)',
              transition: 'background 0.12s, color 0.12s',
              userSelect: 'none',
            }}
          >
            <span style={{ flex: 1 }}>{label}</span>
            {badge != null && (
              <span style={{
                fontFamily: 'DM Mono, monospace', fontSize: 10,
                background: activeView === id ? 'var(--accent)' : 'var(--surface-3)',
                color: activeView === id ? '#fff' : 'var(--text-3)',
                padding: '1px 6px', borderRadius: 20,
              }}>{badge}</span>
            )}
          </div>
        ))}
      </nav>

      <div style={{ flex: 1, overflowY: 'auto', padding: '12px 8px 0' }}>
        <div className="sec-label" style={{ padding: '0 10px', marginBottom: 6 }}>On this network</div>
        {peers.length === 0 && (
          <p style={{ padding: '8px 10px', fontSize: 12, color: 'var(--text-3)' }}>Scanning…</p>
        )}
        {peers.map(p => (
          <div key={p.Hostname} style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '7px 10px', borderRadius: 'var(--r-sm)' }}>
            <div style={pulseStyle('var(--watch)')} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: 12.5, fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.DisplayName || p.Hostname}</div>
              <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10, color: 'var(--text-3)' }}>{p.Addr}</div>
            </div>
          </div>
        ))}
      </div>

      <div style={{ padding: '12px 18px', borderTop: '1px solid var(--border-2)', display: 'flex', alignItems: 'center', gap: 9 }}>
        <div style={pulseStyle('var(--watch)')} />
        <div>
          <div style={{ fontSize: 11.5, fontWeight: 600 }}>{config.DisplayName || config.DeviceID}</div>
          <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10, color: 'var(--text-3)' }}>:{config.Port}</div>
        </div>
      </div>
    </aside>
  );
}
