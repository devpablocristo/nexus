import { useState, useEffect, useRef, useCallback } from 'react'
import { sendChatMessage, fetchCompanionTasks } from '../api'
import { t } from '../i18n'

type ChatMessage = {
  id: string
  author_type: string
  author_id: string
  body: string
  created_at: string
}

type ChatTask = {
  id: string
  title: string
  status: string
  created_at: string
}

export default function Chat({ lang }: { lang: string }) {
  const [taskId, setTaskId] = useState<string | null>(null)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const [conversations, setConversations] = useState<ChatTask[]>([])
  const [loadingConversations, setLoadingConversations] = useState(true)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  useEffect(() => { scrollToBottom() }, [messages])

  // Cargar conversaciones previas (tareas con channel=console)
  const loadConversations = useCallback(() => {
    setLoadingConversations(true)
    fetchCompanionTasks()
      .then((r: { data?: ChatTask[] }) => {
        const tasks = (r.data || []).filter(
          (task: ChatTask) => task.status !== 'done' && task.status !== 'failed'
        )
        setConversations(tasks)
      })
      .catch(() => {})
      .finally(() => setLoadingConversations(false))
  }, [])

  useEffect(() => { loadConversations() }, [loadConversations])

  const handleSend = async () => {
    const msg = input.trim()
    if (!msg || sending) return

    setSending(true)
    setInput('')

    // Optimistic: agregar mensaje del usuario al hilo
    const optimistic: ChatMessage = {
      id: `temp-${Date.now()}`,
      author_type: 'user',
      author_id: 'subscriber',
      body: msg,
      created_at: new Date().toISOString(),
    }
    setMessages((prev) => [...prev, optimistic])

    try {
      const result = await sendChatMessage(msg, taskId || undefined)
      setTaskId(result.task.id)
      setMessages(result.messages || [])
      // Refrescar lista de conversaciones si es nueva
      if (!taskId) {
        loadConversations()
      }
    } catch {
      setMessages((prev) => [
        ...prev,
        {
          id: `err-${Date.now()}`,
          author_type: 'system',
          author_id: 'system',
          body: 'Error al enviar el mensaje. Intentalo de nuevo.',
          created_at: new Date().toISOString(),
        },
      ])
    } finally {
      setSending(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const startNewConversation = () => {
    setTaskId(null)
    setMessages([])
  }

  const selectConversation = async (id: string) => {
    setTaskId(id)
    try {
      // Cargar mensajes enviando mensaje vacío... no, mejor cargar detalle
      const { fetchCompanionTask } = await import('../api')
      const detail = await fetchCompanionTask(id)
      setMessages(detail.messages || [])
    } catch {
      setMessages([])
    }
  }

  const relativeTime = (iso: string): string => {
    const diff = Date.now() - new Date(iso).getTime()
    const mins = Math.floor(diff / 60000)
    if (mins < 1) return lang === 'es' ? 'ahora' : 'now'
    if (mins < 60) return lang === 'es' ? `hace ${mins}m` : `${mins}m ago`
    const hrs = Math.floor(mins / 60)
    if (hrs < 24) return lang === 'es' ? `hace ${hrs}h` : `${hrs}h ago`
    const days = Math.floor(hrs / 24)
    return lang === 'es' ? `hace ${days}d` : `${days}d ago`
  }

  return (
    <div className="flex gap-4 h-[calc(100vh-140px)]">
      {/* Sidebar: conversaciones */}
      <div className="w-64 flex-shrink-0 bg-gray-800 rounded-lg overflow-hidden flex flex-col">
        <div className="p-3 border-b border-gray-700">
          <button
            onClick={startNewConversation}
            className="w-full px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded transition-colors"
          >
            {lang === 'es' ? '+ Nueva conversación' : '+ New conversation'}
          </button>
        </div>
        <div className="flex-1 overflow-y-auto">
          {loadingConversations ? (
            <p className="text-gray-500 text-sm p-3">...</p>
          ) : conversations.length === 0 ? (
            <p className="text-gray-500 text-sm p-3">
              {lang === 'es' ? 'Sin conversaciones' : 'No conversations'}
            </p>
          ) : (
            conversations.map((c) => (
              <button
                key={c.id}
                onClick={() => selectConversation(c.id)}
                className={`w-full text-left px-3 py-2 border-b border-gray-700 hover:bg-gray-700 transition-colors ${
                  taskId === c.id ? 'bg-gray-700' : ''
                }`}
              >
                <p className="text-sm text-white truncate">{c.title}</p>
                <p className="text-xs text-gray-500">{relativeTime(c.created_at)}</p>
              </button>
            ))
          )}
        </div>
      </div>

      {/* Area de chat */}
      <div className="flex-1 flex flex-col bg-gray-800 rounded-lg overflow-hidden">
        {/* Header */}
        <div className="px-4 py-3 border-b border-gray-700">
          <h2 className="text-white font-medium">
            {taskId
              ? conversations.find((c) => c.id === taskId)?.title || t(lang, 'chat')
              : lang === 'es' ? 'Nueva conversación' : 'New conversation'}
          </h2>
        </div>

        {/* Mensajes */}
        <div className="flex-1 overflow-y-auto px-4 py-4 space-y-3">
          {messages.length === 0 && (
            <div className="text-center text-gray-500 mt-12">
              <p className="text-lg mb-2">
                {lang === 'es' ? 'Hola, soy tu compañero de trabajo' : 'Hi, I am your coworker'}
              </p>
              <p className="text-sm">
                {lang === 'es'
                  ? 'Preguntame sobre tu cuenta, configuración, o cualquier cosa del sistema.'
                  : 'Ask me about your account, configuration, or anything about the system.'}
              </p>
            </div>
          )}
          {messages.map((m) => (
            <div
              key={m.id}
              className={`flex ${m.author_type === 'user' ? 'justify-end' : 'justify-start'}`}
            >
              <div
                className={`max-w-[70%] px-4 py-2 rounded-lg text-sm ${
                  m.author_type === 'user'
                    ? 'bg-blue-600 text-white'
                    : m.author_type === 'system'
                    ? 'bg-red-900/50 text-red-300'
                    : 'bg-gray-700 text-gray-200'
                }`}
              >
                <p className="whitespace-pre-wrap">{m.body}</p>
                <p className="text-xs opacity-50 mt-1">{relativeTime(m.created_at)}</p>
              </div>
            </div>
          ))}
          <div ref={messagesEndRef} />
        </div>

        {/* Input */}
        <div className="px-4 py-3 border-t border-gray-700">
          <div className="flex gap-2">
            <textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={
                lang === 'es'
                  ? 'Escribí tu mensaje...'
                  : 'Type your message...'
              }
              rows={1}
              className="flex-1 bg-gray-700 text-white rounded-lg px-4 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-blue-500"
              disabled={sending}
            />
            <button
              onClick={handleSend}
              disabled={sending || !input.trim()}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
            >
              {sending ? '...' : lang === 'es' ? 'Enviar' : 'Send'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
