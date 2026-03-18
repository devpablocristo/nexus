import { useState } from 'react'
import Inbox from './views/Inbox'
import Replay from './views/Replay'
import Learning from './views/Learning'
import Dashboard from './views/Dashboard'
import Policies from './views/Policies'
import { getSavedLang, saveLang, t } from './i18n'

const tabIds = ['inbox', 'policies', 'replay', 'learning', 'dashboard']

export default function App() {
  const [view, setView] = useState('inbox')
  const [selectedRequestId, setSelectedRequestId] = useState(null)
  const [lang, setLang] = useState(getSavedLang)

  const changeLang = (l) => {
    setLang(l)
    saveLang(l)
  }

  const goToReplay = (requestId) => {
    setSelectedRequestId(requestId)
    setView('replay')
  }

  return (
    <div className="min-h-screen">
      <nav className="bg-gray-900 border-b border-gray-800 px-6 py-3 flex items-center gap-8">
        <h1 className="text-lg font-bold text-white tracking-tight">Nexus Review</h1>
        <div className="flex gap-1">
          {tabIds.map((id) => (
            <button
              key={id}
              onClick={() => setView(id)}
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
      <main className="max-w-6xl mx-auto px-6 py-6">
        {view === 'inbox' && <Inbox lang={lang} onViewReplay={goToReplay} />}
        {view === 'replay' && <Replay lang={lang} requestId={selectedRequestId} onBack={() => setView('inbox')} />}
        {view === 'policies' && <Policies lang={lang} />}
        {view === 'learning' && <Learning lang={lang} />}
        {view === 'dashboard' && <Dashboard lang={lang} />}
      </main>
    </div>
  )
}
