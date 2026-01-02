import { Virtuoso, type VirtuosoHandle } from "react-virtuoso"
import { forwardRef, useImperativeHandle, useRef, useCallback, memo } from "react"
import { store } from "../../../wailsjs/go/models"
import { MessageItem } from "./MessageItem"

interface MessageListProps {
  chatId: string
  messages: store.Message[]
  sentMediaCache: React.MutableRefObject<Map<string, string>>
  onReply?: (message: store.Message) => void
  onLoadMore?: () => void
  onPrefetch?: () => void
  onTrimOldMessages?: () => void
  onRangeChanged?: (range: { startIndex: number; endIndex: number }) => void
  firstItemIndex: number
  isLoading?: boolean
  hasMore?: boolean
}

export interface MessageListHandle {
  scrollToBottom: (behavior?: "auto" | "smooth") => void
}

const MemoizedMessageItem = memo(MessageItem)

export const MessageList = forwardRef<MessageListHandle, MessageListProps>(function MessageList(
  {
    chatId,
    messages,
    sentMediaCache,
    onReply,
    onLoadMore,
    onPrefetch,
    onTrimOldMessages,
    onRangeChanged,
    firstItemIndex,
    isLoading,
    hasMore,
  },
  ref,
) {
  const virtuosoRef = useRef<VirtuosoHandle>(null)
  const prefetchTriggeredRef = useRef(false)

  const renderItem = useCallback(
    (_: number, msg: store.Message) => (
      <div className="px-4 py-1">
        <MemoizedMessageItem
          message={msg}
          chatId={chatId}
          sentMediaCache={sentMediaCache}
          onReply={onReply}
        />
      </div>
    ),
    [chatId, onReply, sentMediaCache],
  )

  const scrollToBottom = useCallback(
    (behavior: "auto" | "smooth" = "smooth") => {
      if (virtuosoRef.current && messages.length > 0) {
        virtuosoRef.current.scrollToIndex({
          index: messages.length - 1,
          align: "end",
          behavior,
        })
      }
    },
    [messages.length],
  )

  useImperativeHandle(ref, () => ({ scrollToBottom }))

  const handleStartReached = useCallback(() => {
    if (!isLoading && hasMore && onLoadMore) {
      onLoadMore()
      prefetchTriggeredRef.current = false
    }
  }, [isLoading, hasMore, onLoadMore])

  return (
    <Virtuoso
      ref={virtuosoRef}
      data={messages}
      firstItemIndex={firstItemIndex}
      initialTopMostItemIndex={Math.max(0, messages.length - 1)}
      startReached={handleStartReached}
      followOutput="smooth"
      alignToBottom
      rangeChanged={onRangeChanged}
      increaseViewportBy={{ top: 300, bottom: 0 }}
      className="flex-1 overflow-y-auto bg-repeat virtuoso-scroller"
      style={{ backgroundImage: "url('/assets/images/bg-chat-tile-dark.png')" }}
      itemContent={renderItem}
      components={{
        Header: () => (
          <div className="flex justify-center py-4">
            {isLoading ? (
              <div className="animate-spin h-5 w-5 border-2 border-green-500 rounded-full border-t-transparent" />
            ) : null}
          </div>
        ),
      }}
    />
  )
})
