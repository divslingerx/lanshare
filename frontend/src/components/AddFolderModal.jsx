import { useState } from 'react';
import { OpenFolderDialog } from '../../wailsjs/go/main/App';

export default function AddFolderModal({ onAdd, onClose }) {
  const [path, setPath] = useState('');

  const browse = async () => {
    const selected = await OpenFolderDialog();
    if (selected) setPath(selected);
  };

  const submit = () => {
    if (!path.trim()) return;
    onAdd(path.trim());
    onClose();
  };

  return (
    <div
      style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.65)', backdropFilter: 'blur(6px)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 200 }}
      onClick={e => e.target === e.currentTarget && onClose()}
    >
      <div style={{ background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 14, padding: 24, width: 430, boxShadow: '0 32px 80px rgba(0,0,0,0.6)' }}>
        <div style={{ fontFamily: 'Syne, sans-serif', fontSize: 16, fontWeight: 700, marginBottom: 20 }}>Add Folder</div>

        <div style={{ marginBottom: 22 }}>
          <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--text-2)', textTransform: 'uppercase', letterSpacing: '0.6px', marginBottom: 6 }}>Folder Path</label>
          <div style={{ display: 'flex', gap: 6 }}>
            <input
              value={path}
              onChange={e => setPath(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && submit()}
              placeholder="Select or paste a folder path"
              style={{ flex: 1, padding: '9px 12px', background: 'var(--surface-2)', border: '1px solid var(--border)', borderRadius: 'var(--r-sm)', color: 'var(--text-1)', fontFamily: 'DM Mono, monospace', fontSize: 12, outline: 'none' }}
            />
            <button className="btn btn-ghost" onClick={browse}>Browse…</button>
          </div>
        </div>

        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
          <button className="btn btn-ghost" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={submit}>Add Folder</button>
        </div>
      </div>
    </div>
  );
}
