import { useState, useEffect, useCallback } from 'react';
import {
  GetFolders, AddFolder, RemoveFolder, SetFolderMode,
  GetPeers, GetPeerFolders, GetSubscriptions, Subscribe, Unsubscribe,
  GetConfig, UpdateDisplayName, UpdatePort, UpdateBaseStorage,
  OpenFolderDialog,
} from '../../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';

// Wrap EventsOn to return a cleanup function (Wails v2 EventsOn doesn't return one)
function eventsOn(eventName, callback) {
  EventsOn(eventName, callback);
  return () => EventsOff(eventName);
}

async function enrichPeer(p) {
  try {
    const folders = await GetPeerFolders(p.Hostname);
    return { ...p, Folders: folders || [] };
  } catch {
    return { ...p, Folders: [] };
  }
}

export function useAppState() {
  const [folders, setFolders] = useState([]);
  const [peers, setPeers] = useState([]);
  const [subscriptions, setSubscriptions] = useState([]);
  const [config, setConfig] = useState({});
  const [transfers, setTransfers] = useState([]);

  useEffect(() => {
    GetFolders().then(setFolders);
    GetPeers().then(ps => Promise.all(ps.map(enrichPeer)).then(setPeers));
    GetSubscriptions().then(setSubscriptions);
    GetConfig().then(setConfig);

    const offOnline  = eventsOn('peer:online', p => {
      enrichPeer(p).then(enriched =>
        setPeers(ps => [...ps.filter(x => x.Hostname !== enriched.Hostname), enriched])
      );
    });
    const offOffline = eventsOn('peer:offline', h => setPeers(ps => ps.filter(p => p.Hostname !== h)));
    const offChanged = eventsOn('folder:changed', () => GetFolders().then(setFolders));
    const offProgress = eventsOn('transfer:progress', t => {
      setTransfers(ts => {
        const exists = ts.some(x => x.id === t.id);
        if (exists) return ts.map(x => x.id === t.id ? { ...x, ...t } : x);
        return [...ts, { ...t }];
      });
    });
    const offDone = eventsOn('transfer:complete', t => {
      setTransfers(ts => [{ ...t, done: true }, ...ts.filter(x => x.id !== t.id)].slice(0, 20));
    });

    return () => { offOnline(); offOffline(); offChanged(); offProgress(); offDone(); };
  }, []);

  const addFolder = useCallback(async (path, mode) => {
    await AddFolder(path, mode);
    setFolders(await GetFolders());
  }, []);

  const removeFolder = useCallback(async (id) => {
    await RemoveFolder(id);
    setFolders(await GetFolders());
  }, []);

  const setFolderMode = useCallback(async (id, mode) => {
    await SetFolderMode(id, mode);
    setFolders(await GetFolders());
  }, []);

  const subscribe = useCallback(async (peerHostname, folderID, remoteFolder, localDest) => {
    await Subscribe(peerHostname, folderID, remoteFolder || '', localDest || '');
    setSubscriptions(await GetSubscriptions());
  }, []);

  const unsubscribe = useCallback(async (peerHostname, folderID) => {
    await Unsubscribe(peerHostname, folderID);
    setSubscriptions(await GetSubscriptions());
  }, []);

  const updateConfig = useCallback(async (changes) => {
    if (changes.displayName !== undefined) await UpdateDisplayName(changes.displayName);
    if (changes.port !== undefined) await UpdatePort(changes.port);
    if (changes.baseStorage !== undefined) await UpdateBaseStorage(changes.baseStorage);
    setConfig(await GetConfig());
  }, []);

  const openFolderDialog = useCallback(() => OpenFolderDialog(), []);

  return {
    folders, peers, subscriptions, config, transfers,
    addFolder, removeFolder, setFolderMode,
    subscribe, unsubscribe, updateConfig, openFolderDialog,
  };
}
