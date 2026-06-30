import { useCallback, useState } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { loadIdentity, type StoredIdentity } from './store/identity'
import OnboardingPage from './pages/OnboardingPage'
import ChatPage from './pages/ChatPage'

function App() {
  // Sync lazy init — reads localStorage once on mount, no effect needed.
  const [identity, setIdentity] = useState<StoredIdentity | null>(() => loadIdentity())

  // Called by OnboardingPage after registration is complete and identity is
  // already saved to localStorage. Re-reading here updates state synchronously
  // before the next render, so the /chat route sees identity immediately.
  const handleRegistered = useCallback(() => {
    setIdentity(loadIdentity())
  }, [])

  return (
    <Routes>
      <Route
        path="/onboarding"
        element={
          identity
            ? <Navigate to="/chat" replace />
            : <OnboardingPage onRegistered={handleRegistered} />
        }
      />
      <Route
        path="/chat"
        element={
          identity
            ? <ChatPage identity={identity} />
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
