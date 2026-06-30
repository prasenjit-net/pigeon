import { Navigate, Route, Routes } from 'react-router-dom'
import { useIdentity } from './hooks/useIdentity'
import OnboardingPage from './pages/OnboardingPage'
import ChatPage from './pages/ChatPage'

function App() {
  const identity = useIdentity()

  if (identity.status === 'loading') {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-950">
        <svg className="animate-spin h-8 w-8 text-indigo-600" fill="none" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
        </svg>
      </div>
    )
  }

  return (
    <Routes>
      <Route
        path="/onboarding"
        element={
          identity.status === 'ready'
            ? <Navigate to="/chat" replace />
            : <OnboardingPage />
        }
      />
      <Route
        path="/chat"
        element={
          identity.status === 'ready'
            ? <ChatPage identity={identity.identity} />
            : <Navigate to="/onboarding" replace />
        }
      />
      <Route
        path="*"
        element={
          identity.status === 'ready'
            ? <Navigate to="/chat" replace />
            : <Navigate to="/onboarding" replace />
        }
      />
    </Routes>
  )
}

export default App
