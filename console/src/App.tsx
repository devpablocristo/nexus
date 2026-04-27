import { useState } from 'react'
import Inbox from './views/Inbox'
import Home from './views/Home'
import Requests from './views/Requests'
import Learning from './views/Learning'
import Dashboard from './views/Dashboard'
import Policies from './views/Policies'
import Config from './views/Config'
import ActionTypes from './views/ActionTypes'
import Replay from './views/Replay'
import { getSavedLang, saveLang, t } from './i18n'
import { getSavedView, saveView } from './storage'
import { AuthTokenBridge, ProtectedRoute } from './AuthTokenBridge'

// Navegación agrupada por áreas de trabajo (Workspace).
// Tabs companion-specific (chat, tasks, memory, agents, connectors, sandbox)
// se mudaron al proyecto independiente Companion.
const areas = [
  {
    key: 'areaGovernance',
    tabs: ['home', 'inbox', 'requests', 'replay'],
  },
  {
    key: 'areaOperations',
    tabs: ['policies', 'actionTypes'],
  },
  {
    key: 'areaTools',
    tabs: ['learning', 'dashboard'],
  },
  {
    key: 'areaSettings',
    tabs: ['config'],
  },
]

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
      <nav className="bg-gray-900 border-b border-gray-800 px-6 py-3">
        <div className="flex items-center gap-8 mb-2">
          <h1 className="text-lg font-bold text-white tracking-tight">Nexus Governance</h1>
          <div className="ml-auto flex items-center gap-3">
            <AuthTokenBridge />
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
        </div>
        <div className="flex gap-6">
          {areas.map((area) => (
            <div key={area.key} className="flex items-center gap-1">
              <span className="text-xs text-gray-500 uppercase tracking-wider mr-1">
                {t(lang, area.key)}
              </span>
              {area.tabs.map((id) => (
                <button
                  key={id}
                  onClick={() => changeView(id)}
                  className={`px-3 py-1.5 rounded text-sm font-medium transition-colors ${
                    view === id
                      ? 'bg-gray-700 text-white'
                      : 'text-gray-400 hover:text-white hover:bg-gray-800'
                  }`}
                >
                  {t(lang, id)}
                </button>
              ))}
              <span className="text-gray-700 ml-1">|</span>
            </div>
          ))}
        </div>
      </nav>
      <ProtectedRoute>
        <main className="max-w-7xl mx-auto px-6 py-6">
          {view === 'home' && <Home lang={lang} onViewReplay={viewReplay} onViewInbox={() => changeView('inbox')} />}
          {view === 'inbox' && <Inbox lang={lang} onViewReplay={viewReplay} />}
          {view === 'requests' && <Requests lang={lang} />}
          {view === 'replay' && <Replay lang={lang} requestId={replayRequestId} />}
          {view === 'policies' && <Policies lang={lang} />}
          {view === 'actionTypes' && <ActionTypes lang={lang} />}
          {view === 'learning' && <Learning lang={lang} />}
          {view === 'dashboard' && <Dashboard lang={lang} />}
          {view === 'config' && <Config lang={lang} />}
        </main>
      </ProtectedRoute>
    </div>
  )
}
