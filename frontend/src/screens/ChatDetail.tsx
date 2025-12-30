import { useEffect, useState, useRef } from "react"
import { SendMessage, FetchMessages, SendChatPresence } from "../../wailsjs/go/api/Api"
import { store } from "../../wailsjs/go/models"
import { EventsOn } from "../../wailsjs/runtime/runtime"
import { useMessageStore, useUIStore } from "../store"
import { MessageList } from "../components/chat/MessageList"
import { ChatHeader } from "../components/chat/ChatHeader"
import { ChatInput } from "../components/chat/ChatInput"

interface ChatDetailProps {
  chatId: string
  chatName: string
  chatAvatar?: string
  onBack?: () => void
}

export function ChatDetail({ chatId, chatName, chatAvatar, onBack }: ChatDetailProps) {
  const { messages, setMessages } = useMessageStore()
  const { setTypingIndicator, showEmojiPicker, setShowEmojiPicker } = useUIStore()

  const chatMessages = messages[chatId] || []
  const [inputText, setInputText] = useState("")
  const [pastedImage, setPastedImage] = useState<string | null>(null)
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [selectedFileType, setSelectedFileType] = useState<string>("")
  const [typingTimeout, setTypingTimeout] = useState<NodeJS.Timeout | null>(null)

  const messagesEndRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const sentMediaCache = useRef<Map<string, string>>(new Map())
  const emojiPickerRef = useRef<HTMLDivElement>(null)
  const emojiButtonRef = useRef<HTMLButtonElement>(null)

  // Message loading and scrolling
  const loadMessages = () => {
    FetchMessages(chatId)
      .then((msgs: store.Message[]) => {
        setMessages(chatId, msgs || [])
        scrollToBottom(true)
      })
      .catch(console.error)
  }

  const scrollToBottom = (instant = false) => {
    messagesEndRef.current?.scrollIntoView({ behavior: instant ? "auto" : "smooth" })
  }

  // Input handling
  const adjustTextareaHeight = () => {
    const textarea = textareaRef.current
    if (!textarea) return

    textarea.style.height = "auto"
    const scrollHeight = textarea.scrollHeight
    const lineHeight = parseInt(getComputedStyle(textarea).lineHeight)
    const maxHeight = lineHeight * 8
    textarea.style.height = Math.min(scrollHeight, maxHeight) + "px"
  }

  const handleInputChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInputText(e.target.value)
    adjustTextareaHeight()

    setTypingIndicator(chatId, true)

    // Send composing presence
    SendChatPresence(chatId, "composing", "").catch(() => {});

    // Clear existing timeout
    if (typingTimeout) {
      clearTimeout(typingTimeout);
    }

    const timeout = setTimeout(() => {
      SendChatPresence(chatId, "paused", "").catch(() => {});
      setTypingIndicator(chatId, false)
    }, 1000)

    setTypingTimeout(timeout)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault()
      handleSendMessage()
    }
  }

  const handlePaste = async (e: React.ClipboardEvent<HTMLTextAreaElement>) => {
    const items = e.clipboardData?.items
    if (!items) return

    for (const item of items) {
      if (item.type.indexOf("image") !== -1) {
        e.preventDefault()
        const file = item.getAsFile()
        if (!file) continue

        const reader = new FileReader()
        reader.onload = event => {
          setPastedImage(event.target?.result as string)
          setTimeout(adjustTextareaHeight, 0)
        }
        reader.readAsDataURL(file)
        break
      }
    }
  }

  // File handling
  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    setSelectedFile(file)

    if (file.type.startsWith("image/")) setSelectedFileType("image")
    else if (file.type.startsWith("video/")) setSelectedFileType("video")
    else if (file.type.startsWith("audio/")) setSelectedFileType("audio")
    else setSelectedFileType("document")
  }

  const removeSelectedFile = () => {
    setSelectedFile(null)
    setSelectedFileType("")
    if (fileInputRef.current) fileInputRef.current.value = ""
    setTimeout(adjustTextareaHeight, 0)
  }

  // Message sending
  const handleSendMessage = async () => {
    if (!inputText.trim() && !pastedImage && !selectedFile) return

    const textToSend = inputText
    const imageToSend = pastedImage
    const fileToSend = selectedFile
    const fileTypeToSend = selectedFileType

    // Reset state
    setInputText("")
    setPastedImage(null)
    setSelectedFile(null)
    setSelectedFileType("")

    if (textareaRef.current) textareaRef.current.style.height = "auto"
    if (fileInputRef.current) fileInputRef.current.value = ""

    // Create optimistic message
    const tempMsg = createTempMessage(textToSend, imageToSend, fileToSend, fileTypeToSend)
    setMessages(chatId, [...chatMessages, tempMsg])
    setTimeout(scrollToBottom, 100)

    try {
      await sendMessageContent(
        chatId,
        textToSend,
        imageToSend,
        fileToSend,
        fileTypeToSend,
        sentMediaCache,
      )
      loadMessages()
    } catch (err) {
      console.error("Failed to send message:", err)
      setMessages(
        chatId,
        chatMessages.filter((m: any) => m.Info.ID !== tempMsg.Info.ID),
      )
      setInputText(textToSend)
      setPastedImage(imageToSend)
      setSelectedFile(fileToSend)
      setSelectedFileType(fileTypeToSend)
      setTimeout(adjustTextareaHeight, 0)
    }
  }

  // Emoji picker
  const handleEmojiClick = (emoji: string) => {
    setInputText(prev => prev + emoji)
    setShowEmojiPicker(false)
    textareaRef.current?.focus()
    setTimeout(adjustTextareaHeight, 0)
  }
  // Effects
  useEffect(() => {
    loadMessages()
    const unsub = EventsOn("wa:new_message", loadMessages)
    return () => unsub()
  }, [chatId])

  useEffect(() => {
    return () => {
      if (typingTimeout) clearTimeout(typingTimeout)
    }
  }, [typingTimeout])

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        emojiPickerRef.current &&
        !emojiPickerRef.current.contains(event.target as Node) &&
        emojiButtonRef.current &&
        !emojiButtonRef.current.contains(event.target as Node)
      ) {
        setShowEmojiPicker(false)
      }
    }

    if (showEmojiPicker) {
      document.addEventListener("mousedown", handleClickOutside)
    }

    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [showEmojiPicker, setShowEmojiPicker])

  return (
    <div className="flex flex-col h-full bg-[#efeae2] dark:bg-[#0b141a]">
      <ChatHeader chatName={chatName} chatAvatar={chatAvatar} onBack={onBack} />

      <MessageList
        chatId={chatId}
        messages={chatMessages}
        messagesEndRef={messagesEndRef}
        sentMediaCache={sentMediaCache}
      />

      <ChatInput
        inputText={inputText}
        pastedImage={pastedImage}
        selectedFile={selectedFile}
        selectedFileType={selectedFileType}
        showEmojiPicker={showEmojiPicker}
        textareaRef={textareaRef}
        fileInputRef={fileInputRef}
        emojiPickerRef={emojiPickerRef}
        emojiButtonRef={emojiButtonRef}
        onInputChange={handleInputChange}
        onKeyDown={handleKeyDown}
        onPaste={handlePaste}
        onSendMessage={handleSendMessage}
        onFileSelect={handleFileSelect}
        onRemoveFile={removeSelectedFile}
        onEmojiClick={handleEmojiClick}
        onToggleEmojiPicker={() => setShowEmojiPicker(!showEmojiPicker)}
      />
    </div>
  )
}

// Helper functions
function createTempMessage(
  text: string,
  image: string | null,
  file: File | null,
  fileType: string,
): any {
  const tempId = `temp-${Date.now()}`

  let content: any
  if (image) {
    content = { imageMessage: { caption: text, _tempImage: image } }
  } else if (file) {
    content = { [`${fileType}Message`]: { caption: text, _tempFile: file } }
  } else {
    content = { conversation: text }
  }

  return {
    Info: {
      ID: tempId,
      IsFromMe: true,
      Timestamp: new Date().toISOString(),
    },
    Content: content,
  }
}

async function sendMessageContent(
  chatId: string,
  text: string,
  image: string | null,
  file: File | null,
  fileType: string,
  cache: React.MutableRefObject<Map<string, string>>,
) {
  if (image) {
    const base64 = image.split(",")[1]
    const newId = await SendMessage(chatId, {
      type: "image",
      base64Data: base64,
      text,
    })
    if (newId) cache.current.set(newId, image)
  } else if (file) {
    const reader = new FileReader()
    return new Promise<void>((resolve, reject) => {
      reader.onload = async event => {
        try {
          const base64 = (event.target?.result as string).split(",")[1]
          const newId = await SendMessage(chatId, {
            type: fileType,
            base64Data: base64,
            text,
          })
          if (newId) cache.current.set(newId, event.target?.result as string)
          resolve()
        } catch (err) {
          reject(err)
        }
      }
      reader.onerror = reject
      reader.readAsDataURL(file)
    })
  } else {
    await SendMessage(chatId, { type: "text", text })
  }
}
