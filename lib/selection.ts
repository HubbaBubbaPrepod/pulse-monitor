type SelectionStorage = Pick<Storage,'getItem'|'setItem'|'removeItem'>;
const selectionKey = 'pulse.selected-server.v1';
function browserStorage(): SelectionStorage | undefined {
 try { return typeof window === 'undefined' ? undefined : window.localStorage; }
 catch { return undefined; }
}
export function restoreServerSelection(servers:readonly {id:string}[],storage:SelectionStorage|undefined=browserStorage()):string {
 try { const saved = storage?.getItem(selectionKey); if (servers.some(server => server.id === saved)) return saved!; }
 catch { /* Storage is optional. */ }
 return servers[0]?.id ?? '';
}
export function persistServerSelection(id:string|undefined,storage:SelectionStorage|undefined=browserStorage()):void {
 if (id === undefined) return;
 try { if (id) storage?.setItem(selectionKey,id); else storage?.removeItem(selectionKey); }
 catch { /* Monitoring still works when storage is blocked. */ }
}
