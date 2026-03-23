import { useEffect } from 'react'
import { useAuth, UserButton } from '@clerk/react'
import { createClerkTokenProvider } from '@devpablocristo/core-authn/providers/clerk'
import { registerTokenProvider } from '@devpablocristo/core-authn/http/fetch'
import { setClerkTokenGetter } from './api'
import { clerkEnabled } from './auth'

// Registra el token de Clerk en core-authn y en api.ts para que las requests usen Bearer automáticamente.
export function AuthTokenBridge() {
  if (!clerkEnabled) return null
  return <ClerkBridge />
}

function ClerkBridge() {
  const { getToken, isLoaded, isSignedIn } = useAuth()

  useEffect(() => {
    if (!isLoaded) return
    const provider = createClerkTokenProvider(getToken)
    registerTokenProvider(provider)
    setClerkTokenGetter(getToken)
  }, [getToken, isLoaded])

  if (!isLoaded) return null
  if (!isSignedIn) {
    return (
      <div className="flex items-center gap-2 text-sm text-yellow-400">
        <span>Sign in required</span>
      </div>
    )
  }

  return <UserButton afterSignOutUrl="/" />
}

// Componente para proteger rutas — redirige al sign-in si no hay sesión.
export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  if (!clerkEnabled) return <>{children}</>
  return <ClerkProtected>{children}</ClerkProtected>
}

function ClerkProtected({ children }: { children: React.ReactNode }) {
  const { isLoaded, isSignedIn } = useAuth()

  if (!isLoaded) {
    return <div className="text-gray-400 p-6">Loading...</div>
  }
  if (!isSignedIn) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-gray-900">
        <div className="text-center">
          <h2 className="text-xl font-bold text-white mb-4">Nexus Workspace</h2>
          <p className="text-gray-400 mb-6">Sign in to continue</p>
          <a href="/" className="bg-blue-600 hover:bg-blue-700 text-white px-6 py-2 rounded">
            Sign In
          </a>
        </div>
      </div>
    )
  }

  return <>{children}</>
}
