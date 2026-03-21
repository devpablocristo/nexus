import { useState } from 'react'
import Inbox from './views/Inbox'
import Requests from './views/Requests'
import Learning from './views/Learning'
import Dashboard from './views/Dashboard'
import Policies from './views/Policies'
import Config from './views/Config'
import Sandbox from './views/Sandbox'
import ActionTypes from './views/ActionTypes'
import Agents from './views/Agents'
import Replay from './views/Replay'
import { getSavedLang, saveLang, t } from './i18n'
import { getSavedView, saveView } from './storage'

const tabIds = ['inbox', 'requests', 'replay', 'policies', 'actionTypes', 'agents', 'sandbox', 'learning', 'dashboard', 'config']

export default function App() {
  const [view, setView] = useState(getSavedView)
  const [lang, setLang] = useState(getSavedLang)
  const [replayRequestId, setReplayRequestId] = useState<string | null>(null)

  const changeView = (v: string) => {
    setView(v)
    saveView(v)
    if (v !== 'replay') {
      setReplayRequestId(null)
    }
  }

  const changeLang = (l: string) => {
    setLang(l)
    saveLang(l)
  }

  const viewReplay = (requestId: string) => {
    setReplayRequestId(requestId)
    setView('replay')
    saveView('replay')
  }

  return (
    <div className="min-h-screen">
      <nav className="bg-gray-900 border-b border-gray-800 px-6 py-3 flex items-center gap-8">
        <h1 className="text-lg font-bold text-white tracking-tight">Nexus Review</h1>
        <div className="flex gap-1">
          {tabIds.map((id) => (
            <button
              key={id}
              onClick={() => changeView(id)}
              className={`px-4 py-2 rounded text-sm font-medium transition-colors ${
                view === id
                  ? 'bg-gray-700 text-white'
                  : 'text-gray-400 hover:text-white hover:bg-gray-800'
              }`}
            >
              {t(lang, id)}
            </button>
          ))}
        </div>
        <div className="ml-auto flex gap-1">
          {['en', 'es'].map((l) => (
            <button
              key={l}
              onClick={() => changeLang(l)}
              className={`px-2 py-1 rounded text-xs font-medium uppercase transition-colors ${
                lang === l
                  ? 'bg-gray-700 text-white'
                  : 'text-gray-500 hover:text-white hover:bg-gray-800'
              }`}
            >
              {l}
            </button>
          ))}
        </div>
      </nav>
      <main className="max-w-7xl mx-auto px-6 py-6">
        {view === 'inbox' && <Inbox lang={lang} onViewReplay={viewReplay} />}
        {view === 'requests' && <Requests lang={lang} />}
        {view === 'replay' && <Replay lang={lang} requestId={replayRequestId} />}
        {view === 'policies' && <Policies lang={lang} />}
        {view === 'actionTypes' && <ActionTypes lang={lang} />}
        {view === 'agents' && <Agents lang={lang} />}
        {view === 'sandbox' && <Sandbox lang={lang} />}
        {view === 'learning' && <Learning lang={lang} />}
        {view === 'dashboard' && <Dashboard lang={lang} />}
        {view === 'config' && <Config lang={lang} />}
      </main>
    </div>
  )
}
