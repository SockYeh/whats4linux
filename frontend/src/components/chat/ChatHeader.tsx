import { GoBackIcon } from "../../assets/svgs/header_icons"
import { InfoIcon } from "../../assets/svgs/chat_info_icons"

interface ChatHeaderProps {
  chatName: string
  chatSubtitle?: string
  chatAvatar?: string
  onBack?: () => void
  onInfoClick?: () => void
  onCallClick?: () => void
}

const PhoneIcon = () => (
  <svg viewBox="0 0 24 24" width="22" height="22" fill="currentColor">
    <path d="M19.95 21q-3.225 0-6.287-1.438-3.063-1.437-5.425-3.8-2.363-2.362-3.8-5.425Q3 7.275 3 4.05q0-.45.3-.75t.75-.3H8.1q.35 0 .613.213.262.212.337.537l.787 3.45q.05.2-.012.387-.063.188-.238.338l-2.65 2.65q1.15 2 2.8 3.65t3.675 2.775l2.65-2.625q.15-.15.35-.225.2-.075.4-.025l3.375.8q.35.075.563.337.212.263.212.613v4.05q0 .45-.3.75t-.75.3Z" />
  </svg>
)

export function ChatHeader({
  chatName,
  chatSubtitle,
  chatAvatar,
  onBack,
  onInfoClick,
  onCallClick,
}: ChatHeaderProps) {
  return (
    <div className="flex items-center justify-between p-3 bg-light-secondary dark:bg-dark-bg border-b border-gray-300 dark:border-white/5">
      <div className="flex items-center gap-3 min-w-0 flex-1">
        {onBack && (
          <button onClick={onBack} className="mr-4 md:hidden">
            <GoBackIcon />
          </button>
        )}
        <div className="flex items-center gap-3 cursor-pointer min-w-0" onClick={onInfoClick}>
          <div className="w-10 h-10 shrink-0 rounded-full bg-gray-300 dark:bg-gray-600 flex items-center justify-center text-white font-bold overflow-hidden">
            {chatAvatar ? (
              <img src={chatAvatar} alt={chatName} className="w-full h-full object-cover" />
            ) : (
              chatName.substring(0, 1).toUpperCase()
            )}
          </div>
          <div className="min-w-0">
            <h2 className="text-[16px] font-medium text-gray-800 dark:text-gray-100 truncate">
              {chatName}
            </h2>
            {chatSubtitle && (
              <div className="text-xs text-gray-500 dark:text-[#8696a0] truncate">
                {chatSubtitle}
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="flex items-center shrink-0">
        {onCallClick && (
          <button
            onClick={onCallClick}
            className="p-2 hover:bg-gray-200 dark:hover:bg-dark-tertiary rounded-full transition-colors text-gray-600 dark:text-gray-300"
            aria-label="Voice call"
            title="Voice call"
          >
            <PhoneIcon />
          </button>
        )}
        <button
          onClick={onInfoClick}
          className="p-1 hover:bg-gray-200 dark:hover:bg-dark-tertiary rounded-full transition-colors"
          aria-label="Chat info"
        >
          <InfoIcon />
        </button>
      </div>
    </div>
  )
}
