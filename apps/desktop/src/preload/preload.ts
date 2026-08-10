import { contextBridge, ipcRenderer } from 'electron'

contextBridge.exposeInMainWorld('byterush', {
  getConfig: () => ipcRenderer.invoke('byterush:config'),
  chooseDirectory: () => ipcRenderer.invoke('byterush:choose-directory'),
  revealFile: (p: string) => ipcRenderer.invoke('byterush:reveal-file', p),
  notify: (title: string, body: string) => ipcRenderer.invoke('byterush:notify', title, body),
})
