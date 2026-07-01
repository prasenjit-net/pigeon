import { useCallback, useState } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { clearIdentity, loadIdentity, type StoredIdentity } from './store/identity'
import OnboardingPage from './pages/OnboardingPage'
import ChatPage from './pages/ChatPage'

function App() {
  // Sync lazy init — reads localStorage once on mount, no effect needed.
  const [identity, setIdentity] = useState<StoredIdentity | null>(() => loadIdentity())
  const [logoutReason, setLogoutReason] = useState<string | null>(null)

  // Called by OnboardingPage after registration is complete and identity is
  // already saved to localStorage. Re-reading here updates state synchronously
  // before the next render, so the /chat route sees identity immediately.
  const handleRegistered = useCallback(() => {
    setLogoutReason(null)
    setIdentity(loadIdentity())
  }, [])

  const handleLogout = useCallback((reason?: string) => {
    clearIdentity()
    setIdentity(null)
    setLogoutReason(reason ?? null)
  }, [])

  return (
    <Routes>
      <Route
        path="/onboarding"
        element={
          identity
            ? <Navigate to="/chat" replace />
            : <OnboardingPage onRegistered={handleRegistered} reason={logoutReason} />
        }
      />
      <Route
        path="/chat"
        element={
          identity
            ? <ChatPage identity={identity} onLogout={handleLogout} />
            : <Navigate to="/onboarding" replace />
        }
      />
      <Route
        path="*"
        element={<Navigate to={identity ? '/chat' : '/onboarding'} replace />}
      />
    </Routes>
  )
}

export default App
