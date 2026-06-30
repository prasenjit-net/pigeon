const MAX_MESSAGES = 500

export interface PersistedMsg {
  id: string          // server-assigned UUID after ack; clientId before ack
  clientId?: string   // temp id assigned at send time, for ack matching
  direction: 'sent' | 'received'
  text: string        // decrypted plaintext
  timestamp: number   // Unix milliseconds
  status: 'delivered' | 'queued' | 'failed'
  error?: string
}

interface ConversationStore {
  messages: PersistedMsg[]
  lastReadAt: number  // Unix ms; messages received after this are "unread"
}

function storageKey(otherUserId: string): string {
  return `pigeon.conv.${otherUserId}`
}

export function loadConversation(otherUserId: string): ConversationStore {
  try {
    const raw = localStorage.getItem(storageKey(otherUserId))
    if (!raw) return { messages: [], lastReadAt: 0 }
    return JSON.parse(raw) as ConversationStore
  } catch {
    return { messages: [], lastReadAt: 0 }
  }
}

function saveConversation(otherUserId: string, store: ConversationStore): void {
  const trimmed =
    store.messages.length > MAX_MESSAGES
      ? { ...store, messages: store.messages.slice(-MAX_MESSAGES) }
      : store
  localStorage.setItem(storageKey(otherUserId), JSON.stringify(trimmed))
}

// appendMessage adds a message to the conversation, deduplicating by id.
export function appendMessage(otherUserId: string, msg: PersistedMsg): void {
  const store = loadConversation(otherUserId)
  if (store.messages.some((m) => m.id === msg.id)) return
  saveConversation(otherUserId, { ...store, messages: [...store.messages, msg] })
}

// updateMessageById patches a message matched by id.
export function updateMessageById(
  otherUserId: string,
  id: string,
  patch: Partial<PersistedMsg>,
): void {
  const store = loadConversation(otherUserId)
  const messages = store.messages.map((m) => (m.id === id ? { ...m, ...patch } : m))
  saveConversation(otherUserId, { ...store, messages })
}

// markRead sets lastReadAt to now for the conversation.
export function markRead(otherUserId: string): void {
  const store = loadConversation(otherUserId)
  saveConversation(otherUserId, { ...store, lastReadAt: Date.now() })
}

// unreadCount returns the number of received messages after lastReadAt.
export function unreadCount(otherUserId: string): number {
  const { messages, lastReadAt } = loadConversation(otherUserId)
  return messages.filter((m) => m.direction === 'received' && m.timestamp > lastReadAt).length
}

export function clearConversation(otherUserId: string): void {
  localStorage.removeItem(storageKey(otherUserId))
}
