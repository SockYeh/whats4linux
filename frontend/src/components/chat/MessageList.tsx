import { forwardRef, useImperativeHandle, useRef, useCallback, memo, useEffect } from "react"
import { store } from "../../../wailsjs/go/models"
import { MessageItem } from "./MessageItem"
import clsx from "clsx"

interface MessageListProps {
  chatId: string
  messages: store.Message[]
  sentMediaCache: React.MutableRefObject<Map<string, string>>
  onReply?: (message: store.Message) => void
  onQuotedClick?: (messageId: string) => void
  onLoadMore?: () => void
  onPrefetch?: () => void
  onTrimOldMessages?: () => void
  onRangeChanged?: (range: { startIndex: number; endIndex: number }) => void
  onAtBottomChange?: (atBottom: boolean) => void
  firstItemIndex: number
  isLoading?: boolean
  hasMore?: boolean
  highlightedMessageId?: string | null
}

export interface MessageListHandle {
  scrollToBottom: (behavior?: "auto" | "smooth") => void
  scrollToMessage: (messageId: string) => void
}

const MemoizedMessageItem = memo(MessageItem)

export const MessageList = forwardRef<MessageListHandle, MessageListProps>(function MessageList(
  {
    chatId,
    messages,
    sentMediaCache,
    onReply,
    onQuotedClick,
    onLoadMore,
    onPrefetch,
    onTrimOldMessages,
    onRangeChanged,
    onAtBottomChange,
    firstItemIndex,
    isLoading,
    hasMore,
    highlightedMessageId,
  },
  ref,
) {
  const containerRef = useRef<HTMLDivElement | null>(null)

  const renderItem = useCallback(
    (_: number, msg: store.Message) => {
      const isHighlighted = highlightedMessageId === msg.Info?.ID
      return (
        <div
          className={clsx("px-4 py-1 transition-colors duration-500", {
            "bg-green-200/50 dark:bg-green-500/30": isHighlighted,
          })}
        >
          <MemoizedMessageItem
            message={msg}
            chatId={chatId}
            sentMediaCache={sentMediaCache}
            onReply={onReply}
            onQuotedClick={onQuotedClick}
          />
        </div>
      )
    },
    [chatId, onReply, onQuotedClick, sentMediaCache, highlightedMessageId],
  )

  const scrollToBottom = useCallback((behavior: "auto" | "smooth" = "smooth") => {
    const el = containerRef.current
    if (el) {
      const top = el.scrollHeight - el.clientHeight
      try {
        el.scrollTo({ top, behavior })
      } catch {
        el.scrollTop = top
      }
    }
  }, [])

  const scrollToMessage = useCallback((messageId: string) => {
    const el = containerRef.current
    if (!el) return

    const messageElement = el.querySelector(`[data-message-id="${messageId}"]`) as HTMLElement
    if (messageElement) {
      messageElement.scrollIntoView({ behavior: "smooth", block: "center" })
    }
  }, [])

  useImperativeHandle(ref, () => ({ scrollToBottom, scrollToMessage }))

  useEffect(() => {
    // Scroll to bottom on mount
    if (containerRef.current && messages.length > 0) {
      const el = containerRef.current
      el.scrollTop = el.scrollHeight
    }
  }, [])

  const onScroll = useCallback(
    (e: React.UIEvent<HTMLDivElement>) => {
      const el = e.currentTarget
      // Trigger load more when ~2 messages are left above viewport (assuming ~100px per message)
      if (el.scrollTop <= 200 && !isLoading && hasMore && onLoadMore) {
        onLoadMore()
      }
      const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 5
      onAtBottomChange?.(atBottom)
    },
    [isLoading, hasMore, onLoadMore, onAtBottomChange],
  )

  return (
    <div
      ref={containerRef}
      onScroll={onScroll}
      className="h-full overflow-y-auto bg-repeat virtuoso-scroller"
      style={{ backgroundImage: "url('/assets/images/bg-chat-tile-dark.png')" }}
    >
      <div className="flex justify-center py-4">
        {isLoading ? (
          <div className="animate-spin h-5 w-5 border-2 border-green-500 rounded-full border-t-transparent" />
        ) : null}
      </div>
      {messages.map(msg => (
        <div key={msg.Info.ID} data-message-id={msg.Info.ID} className="px-4 py-1">
          <MemoizedMessageItem
            message={msg}
            chatId={chatId}
            sentMediaCache={sentMediaCache}
            onReply={onReply}
          />
        </div>
      ))}
    </div>
  )
})
