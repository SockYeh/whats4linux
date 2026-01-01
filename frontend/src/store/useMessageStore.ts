import { create } from "zustand"

interface MessageStore {
  messages: Record<string, any[]>
  activeChatId: string | null
  setActiveChatId: (chatId: string) => void
  setMessages: (chatId: string, messages: any[]) => void
  addMessage: (chatId: string, message: any) => void
  prependMessages: (chatId: string, messages: any[]) => void
  updateMessage: (chatId: string, message: any) => void
  clearMessages: (chatId: string) => void
  trimOldMessages: (chatId: string, keepCount: number) => void
}

export const useMessageStore = create<MessageStore>((set, get) => ({
  messages: {},
  activeChatId: null,

  setActiveChatId: chatId => {
    const state = get()
    const prevChatId = state.activeChatId

    // When switching chats, trim the previous chat's messages to just the last one
    if (prevChatId && prevChatId !== chatId && state.messages[prevChatId]?.length > 1) {
      const prevMessages = state.messages[prevChatId]
      set(s => ({
        messages: {
          ...s.messages,
          [prevChatId]: [prevMessages[prevMessages.length - 1]], // Keep only last message
        },
        activeChatId: chatId,
      }))
    } else {
      set({ activeChatId: chatId })
    }
  },

  setMessages: (chatId, messages) =>
    set(state => ({
      messages: {
        ...state.messages,
        [chatId]: messages,
      },
    })),

  addMessage: (chatId, message) =>
    set(state => {
      const existing = state.messages[chatId] || []
      const newMessages = [...existing, message]
      return {
        messages: {
          ...state.messages,
          [chatId]: newMessages,
        },
      }
    }),

  prependMessages: (chatId, messages) =>
    set(state => {
      const existing = state.messages[chatId] || []
      const combined = [...messages, ...existing]
      return {
        messages: {
          ...state.messages,
          [chatId]: combined,
        },
      }
    }),

  // Update or add a message based on its ID (for WhatsMeow events)
  updateMessage: (chatId, message) =>
    set(state => {
      const existing = state.messages[chatId] || []
      const msgId = message.Info?.ID
      const idx = existing.findIndex((m: any) => m.Info?.ID === msgId)

      if (idx >= 0) {
        // Update existing message
        const updated = [...existing]
        updated[idx] = message
        return { messages: { ...state.messages, [chatId]: updated } }
      } else {
        // Add new message
        const newMessages = [...existing, message]
        return {
          messages: {
            ...state.messages,
            [chatId]: newMessages,
          },
        }
      }
    }),

  // Trim old messages from the top, keeping the most recent ones
  trimOldMessages: (chatId, keepCount) =>
    set(state => {
      const existing = state.messages[chatId] || []
      if (existing.length <= keepCount) return state

      return {
        messages: {
          ...state.messages,
          [chatId]: existing.slice(-keepCount),
        },
      }
    }),

  clearMessages: chatId =>
    set(state => {
      const newMessages = { ...state.messages }
      delete newMessages[chatId]
      return { messages: newMessages }
    }),
}))
