import { useState, useEffect } from 'react';

export default function SettingsView({ config, onUpdate, theme, onThemeChange }) {
  const [displayName, setDisplayName] = useState('');
  const [port, setPort] = useState('');
  const [baseStorage, setBaseStorage] = useState('');

  useEffect(() => {
    setDisplayName(config.display_name || '');
    setPort(String(config.port || 47990));
    setBaseStorage(config.base_storage || '');
  }, [config]);

  const save = () => {
    const p = parseInt(port, 10);
    onUpdate({
      displayName,
      port: isNaN(p) ? config.port : p,
      baseStorage,
    });
  };

  const field = (label, value, onChange, placeholder) => (
    <div style={{ marginBottom: 20 }}>
      <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--text-2)', textTransform: 'uppercase', letterSpacing: '0.6px', marginBottom: 6 }}>{label}</label>
      <input
        value={value}
        onChange={e => onChange(e.target.value)}
        placeholder={placeholder}
        style={{ width: '100%', padding: '9px 12px', background: 'var(--surface-2)', border: '1px solid var(--border)', borderRadius: 'var(--r-sm)', color: 'var(--text-1)', fontFamily: 'DM Mono, monospace', fontSize: 12, outline: 'none' }}
      />
    </div>
  );

  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <div style={{ padding: '18px 24px', borderBottom: '1px solid var(--border)', background: 'var(--surface)' }}>
        <div style={{ fontFamily: 'Syne, sans-serif', fontSize: 17, fontWeight: 700, letterSpacing: '-0.4px' }}>Settings</div>
        <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10.5, color: 'var(--text-3)', marginTop: 2 }}>Device ID: {config.device_id}</div>
      </div>

      <div style={{ flex: 1, overflowY: 'auto', padding: '24px', maxWidth: 480 }}>
        {field('Display Name', displayName, setDisplayName, config.device_id)}
        {field('Port', port, setPort, '47990')}
        {field('Base Storage Path', baseStorage, setBaseStorage, '~/filehub')}

        <button className="btn btn-primary" onClick={save}>Save Settings</button>

        <div style={{ marginTop: 28, marginBottom: 20 }}>
          <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--text-2)', textTransform: 'uppercase', letterSpacing: '0.6px', marginBottom: 12 }}>Appearance</label>
          <div style={{ display: 'flex', gap: 8 }}>
            {['light', 'dark'].map(t => (
              <button
                key={t}
                onClick={() => onThemeChange(t)}
                style={{
                  flex: 1,
                  padding: '10px 0',
                  borderRadius: 'var(--r-sm)',
                  border: `1px solid ${theme === t ? 'var(--accent)' : 'var(--border)'}`,
                  background: theme === t ? 'var(--accent-bg)' : 'var(--surface-2)',
                  color: theme === t ? 'var(--accent)' : 'var(--text-2)',
                  fontFamily: 'DM Sans, sans-serif',
                  fontSize: 13,
                  fontWeight: 500,
                  cursor: 'pointer',
                  transition: 'all 0.12s',
                  textTransform: 'capitalize',
                }}
              >
                {t === 'dark' ? '🌙 Dark' : '☀️ Light'}
              </button>
            ))}
          </div>
        </div>

        <div style={{ marginTop: 16, padding: 16, background: 'var(--surface-2)', border: '1px solid var(--border-2)', borderRadius: 'var(--r-sm)' }}>
          <div className="sec-label" style={{ marginBottom: 8 }}>About</div>
          <div style={{ fontSize: 12, color: 'var(--text-3)', lineHeight: 1.7 }}>
            filehub v0.1 — LAN file sharing<br />
            Port changes take effect on next restart.
          </div>
        </div>
      </div>
    </div>
  );
}
