import { useState } from 'react';
import FolderCard from './FolderCard';
import AddFolderModal from './AddFolderModal';

export default function FolderList({ folders, onAddFolder, onRemoveFolder, onSetMode }) {
  const [showModal, setShowModal] = useState(false);

  const watching = folders.filter(f => f.mode === 'watch').length;
  const shared   = folders.filter(f => f.mode === 'shared').length;

  return (
    <>
      <div style={{ padding: '18px 24px', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', gap: 12, background: 'var(--surface)' }}>
        <div>
          <div style={{ fontFamily: 'Syne, sans-serif', fontSize: 17, fontWeight: 700, letterSpacing: '-0.4px' }}>My Folders</div>
          <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10.5, color: 'var(--text-3)', marginTop: 2 }}>
            {folders.length} folder{folders.length !== 1 ? 's' : ''} · {watching} watching · {shared} shared
          </div>
        </div>
        <div style={{ flex: 1 }} />
        <button className="btn btn-primary" onClick={() => setShowModal(true)}>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
          Add Folder
        </button>
      </div>

      <div style={{ flex: 1, overflowY: 'auto', padding: '16px 24px', display: 'flex', flexDirection: 'column', gap: 7 }}>
        {folders.length === 0 && (
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 10, color: 'var(--text-3)', textAlign: 'center', padding: 48 }}>
            <div style={{ width: 52, height: 52, borderRadius: 14, background: 'var(--surface-2)', display: 'flex', alignItems: 'center', justifyContent: 'center', marginBottom: 4 }}>
              <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="var(--text-3)" strokeWidth="1.5"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>
            </div>
            <div style={{ fontFamily: 'Syne, sans-serif', fontSize: 15, fontWeight: 700, color: 'var(--text-2)' }}>No folders added</div>
            <div style={{ fontSize: 13, color: 'var(--text-3)', maxWidth: 240, lineHeight: 1.65 }}>
              Add a folder to start sharing files with other devices on your local network.
            </div>
          </div>
        )}
        {folders.map(f => (
          <FolderCard key={f.id} folder={f} onRemove={onRemoveFolder} onSetMode={onSetMode} />
        ))}
      </div>

      {showModal && (
        <AddFolderModal
          onAdd={onAddFolder}
          onClose={() => setShowModal(false)}
        />
      )}
    </>
  );
}
