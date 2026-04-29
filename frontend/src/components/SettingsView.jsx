import { useState, useEffect } from 'react';

export default function SettingsView({ config, onUpdate }) {
  const [displayName, setDisplayName] = useState('');
  const [port, setPort] = useState('');
  const [baseStorage, setBaseStorage] = useState('');

  useEffect(() => {
    setDisplayName(config.DisplayName || '');
    setPort(String(config.Port || 47990));
    setBaseStorage(config.BaseStorage || '');
  }, [config]);

  const save = () => {
    const p = parseInt(port, 10);
    onUpdate({
      displayName,
      port: isNaN(p) ? config.Port : p,
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
        <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10.5, color: 'var(--text-3)', marginTop: 2 }}>Device ID: {config.DeviceID}</div>
      </div>

      <div style={{ flex: 1, overflowY: 'auto', padding: '24px', maxWidth: 480 }}>
        {field('Display Name', displayName, setDisplayName, config.DeviceID)}
        {field('Port', port, setPort, '47990')}
        {field('Base Storage Path', baseStorage, setBaseStorage, '~/filehub')}

        <button className="btn btn-primary" onClick={save}>Save Settings</button>

        <div style={{ marginTop: 32, padding: 16, background: 'var(--surface-2)', border: '1px solid var(--border-2)', borderRadius: 'var(--r-sm)' }}>
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
