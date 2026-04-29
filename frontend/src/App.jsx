import { useState } from 'react';
import './App.css';
import Sidebar from './components/Sidebar';
import FolderList from './components/FolderList';
import ActivityPanel from './components/ActivityPanel';
import NetworkView from './components/NetworkView';
import SettingsView from './components/SettingsView';
import { useAppState } from './hooks/useAppState';

export default function App() {
  const [view, setView] = useState('folders');
  const state = useAppState();

  return (
    <div style={{ display: 'flex', height: '100vh', overflow: 'hidden' }}>
      <Sidebar
        activeView={view}
        onNavChange={setView}
        peers={state.peers}
        config={state.config}
        folderCount={state.folders.length}
      />

      <main style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', background: 'transparent' }}>
        {view === 'folders' && (
          <FolderList
            folders={state.folders}
            onAddFolder={state.addFolder}
            onRemoveFolder={state.removeFolder}
            onSetMode={state.setFolderMode}
          />
        )}
        {view === 'network' && (
          <NetworkView
            peers={state.peers}
            subscriptions={state.subscriptions}
            onSubscribe={state.subscribe}
            onUnsubscribe={state.unsubscribe}
          />
        )}
        {view === 'settings' && (
          <SettingsView config={state.config} onUpdate={state.updateConfig} />
        )}
      </main>

      <ActivityPanel transfers={state.transfers} />
    </div>
  );
}
