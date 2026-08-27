import { contextBridge, ipcRenderer } from 'electron'

contextBridge.exposeInMainWorld('byterush', {
  getConfig: () => ipcRenderer.invoke('byterush:config'),
  showWindow: () => ipcRenderer.invoke('byterush:show-window'),
  chooseDirectory: () => ipcRenderer.invoke('byterush:choose-directory'),
  revealFile: (p: string) => ipcRenderer.invoke('byterush:reveal-file', p),
  notify: (title: string, body: string) => ipcRenderer.invoke('byterush:notify', title, body),
})
