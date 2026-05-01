export default function NetworkView({ peers, subscriptions, onSubscribe, onUnsubscribe, onRefresh }) {
  const isSubscribed = (peerHostname, folderID) =>
    subscriptions.some(s => s.peer_hostname === peerHostname && s.folder_id === folderID);

  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <div style={{ padding: '18px 24px', borderBottom: '1px solid var(--border)', background: 'var(--surface)', display: 'flex', alignItems: 'center' }}>
        <div>
          <div style={{ fontFamily: 'Syne, sans-serif', fontSize: 17, fontWeight: 700, letterSpacing: '-0.4px' }}>Network</div>
          <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10.5, color: 'var(--text-3)', marginTop: 2 }}>{peers.length} device{peers.length !== 1 ? 's' : ''} found</div>
        </div>
        <div style={{ flex: 1 }} />
        <button className="btn btn-ghost" onClick={onRefresh} style={{ fontSize: 12, gap: 5 }}>
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
          Refresh
        </button>
      </div>

      <div style={{ flex: 1, overflowY: 'auto', padding: '16px 24px', display: 'flex', flexDirection: 'column', gap: 12 }}>
        {peers.length === 0 && (
          <p style={{ color: 'var(--text-3)', fontSize: 13, marginTop: 24, textAlign: 'center' }}>
            No other filehub devices found on this network.
          </p>
        )}
        {peers.map(peer => {
          const folders = (peer.Folders || []).filter(f => !f.disabled);
          return (
            <div key={peer.Hostname} style={{ background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 'var(--r)', overflow: 'hidden' }}>
              <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border-2)', display: 'flex', alignItems: 'center', gap: 10 }}>
                <div style={{ width: 7, height: 7, borderRadius: '50%', background: 'var(--accent)', flexShrink: 0 }} />
                <div>
                  <div style={{ fontWeight: 600, fontSize: 13 }}>{peer.DisplayName || peer.Hostname}</div>
                  <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10, color: 'var(--text-3)' }}>{peer.Addr}:{peer.Port}</div>
                </div>
              </div>
              {folders.map(f => (
                <div key={f.id} style={{ padding: '10px 16px', display: 'flex', alignItems: 'center', gap: 12, borderBottom: '1px solid var(--border-2)' }}>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" strokeWidth="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>
                  <span style={{ fontFamily: 'DM Mono, monospace', fontSize: 11, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={f.path}>{f.path}</span>
                  <button
                    className={isSubscribed(peer.Hostname, f.id) ? 'btn btn-ghost' : 'btn btn-primary'}
                    style={{ fontSize: 11, padding: '4px 10px' }}
                    onClick={() => isSubscribed(peer.Hostname, f.id)
                      ? onUnsubscribe(peer.Hostname, f.id)
                      : onSubscribe(peer.Hostname, f.id, f.path, '')}
                  >
                    {isSubscribed(peer.Hostname, f.id) ? 'Unsubscribe' : 'Subscribe'}
                  </button>
                </div>
              ))}
              {folders.length === 0 && (
                <div style={{ padding: '10px 16px', fontSize: 12, color: 'var(--text-3)' }}>No folders being shared from this device.</div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
