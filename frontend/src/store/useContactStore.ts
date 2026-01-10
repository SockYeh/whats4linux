import { create } from "zustand"
import { immer } from "zustand/middleware/immer"
import { GetContact } from "../../wailsjs/go/api/Api"
import { types } from "../../wailsjs/go/models"

interface ContactStore {
  contacts: Record<string, { name: string; timestamp: number }>
  getContactName: (jid: types.JID) => Promise<string>
  disposeCache: () => void
}

export const useContactStore = create<ContactStore>()(
  immer((set, get) => ({
    contacts: {},

    getContactName: async jid => {
      const userId = jid.User

      const cached = get().contacts[userId]
      if (cached) return cached.name

      try {
        const contact = await GetContact(jid)
        const displayName = contact.full_name || contact.push_name || userId

        set(state => {
          state.contacts[userId] = {
            name: displayName,
            timestamp: Date.now(),
          }
        })
        return displayName
      } catch {
        return userId
      }
    },

    disposeCache: () =>
      set(state => {
        state.contacts = {}
      }),
  })),
)
