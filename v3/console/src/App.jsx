import { useState } from 'react'
import Inbox from './views/Inbox'
import Requests from './views/Requests'
import Learning from './views/Learning'
import Dashboard from './views/Dashboard'
import Policies from './views/Policies'
import Config from './views/Config'
import SimulatePanel from './components/SimulatePanel'
import { getSavedLang, saveLang, t } from './i18n'

const tabIds = ['inbox', 'requests', 'policies', 'learning', 'dashboard', 'config']

export default function App() {
  const [view, setView] = useState(() => localStorage.getItem('nexus-review-tab') || 'inbox')
  const [lang, setLang] = useState(getSavedLang)
  const [simOpen, setSimOpen] = useState(false)

  const changeView = (v) => {
    setView(v)
    localStorage.setItem('nexus-review-tab', v)
  }

  const changeLang = (l) => {
    setLang(l)
    saveLang(l)
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
      <main className="max-w-6xl mx-auto px-6 py-6">
        {view === 'inbox' && <Inbox lang={lang} />}
        {view === 'requests' && <Requests lang={lang} />}
        {view === 'policies' && <Policies lang={lang} />}
        {view === 'learning' && <Learning lang={lang} />}
        {view === 'dashboard' && <Dashboard lang={lang} />}
        {view === 'config' && <Config lang={lang} />}
      </main>

      {/* Botón flotante Simulate */}
      <button
        onClick={() => setSimOpen(true)}
        className="fixed bottom-6 right-6 px-5 py-3 bg-blue-600 text-white rounded-full shadow-lg shadow-blue-600/30 hover:bg-blue-500 transition-colors text-sm font-medium flex items-center gap-2 z-30"
      >
        <span>⚡</span> {t(lang, 'simulate')}
      </button>

      {/* Panel lateral Simulate */}
      <SimulatePanel lang={lang} open={simOpen} onClose={() => setSimOpen(false)} />
    </div>
  )
}
