import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { logInfo } from "../../lib/logger";
import "./chat-styles";

// Mock message interface for now
interface ChatMessage {
  id: number;
  senderId: number;
  senderName: string;
  message: string;
  timestamp: Date;
  isCurrentUser: boolean;
}

// Mock data for development
const mockMessages: ChatMessage[] = [
  {
    id: 1,
    senderId: 2,
    senderName: "Mom",
    message: "Don't forget we have dinner with grandparents tonight!",
    timestamp: new Date(Date.now() - 3600000), // 1 hour ago
    isCurrentUser: false,
  },
  {
    id: 2,
    senderId: 1,
    senderName: "You",
    message: "I'll be there at 6 PM. Should I bring anything?",
    timestamp: new Date(Date.now() - 3300000), // 55 minutes ago
    isCurrentUser: true,
  },
  {
    id: 3,
    senderId: 3,
    senderName: "Dad",
    message: "Maybe some flowers for grandma? She loves those daisies from the garden center.",
    timestamp: new Date(Date.now() - 3000000), // 50 minutes ago
    isCurrentUser: false,
  },
  {
    id: 4,
    senderId: 1,
    senderName: "You",
    message: "Great idea! I'll stop by on my way over.",
    timestamp: new Date(Date.now() - 2700000), // 45 minutes ago
    isCurrentUser: true,
  },
  {
    id: 5,
    senderId: 4,
    senderName: "Emma",
    message: "Can I invite my friend Sarah? She's been wanting to meet everyone.",
    timestamp: new Date(Date.now() - 1800000), // 30 minutes ago
    isCurrentUser: false,
  },
];

interface ChatData {
  messages: ChatMessage[];
}

type MessageForm = {
  message: string;
  sending: boolean;
};

const useMessageForm = vlens.declareHook(
  (): MessageForm => ({
    message: "",
    sending: false,
  })
);

const useChatState = vlens.declareHook(
  (): { messages: ChatMessage[] } => ({
    messages: mockMessages,
  })
);

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<ChatData>({ messages: [] });
  }

  // For now, return mock data
  // In the future, this would call server.GetChatMessages({})
  return rpc.ok<ChatData>({ messages: mockMessages });
}

export function view(route: string, prefix: string, data: ChatData): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="chat-container">
        <ChatPage user={currentAuth} data={data} />
      </main>
      <Footer />
    </div>
  );
}

interface ChatPageProps {
  user: any; // AuthCache type
  data: ChatData;
}

const ChatPage = ({ user, data }: ChatPageProps) => {
  const messageForm = useMessageForm();
  const chatState = useChatState();

  // Initialize chat state with data from server
  if (data.messages && chatState.messages.length === mockMessages.length) {
    chatState.messages = data.messages;
  }

  const handleSendMessage = async (e: Event) => {
    e.preventDefault();

    if (!messageForm.message.trim() || messageForm.sending) {
      return;
    }

    const newMessage: ChatMessage = {
      id: Date.now(), // Simple ID for now
      senderId: user.id,
      senderName: "You",
      message: messageForm.message.trim(),
      timestamp: new Date(),
      isCurrentUser: true,
    };

    messageForm.sending = true;
    vlens.scheduleRedraw();

    try {
      // For now, just add to local state
      // In the future, this would call server.SendMessage({})
      chatState.messages = [...chatState.messages, newMessage];
      messageForm.message = "";

      logInfo("ui", "Message sent", { messageId: newMessage.id });

      // Scroll to bottom after sending
      setTimeout(() => {
        const messagesContainer = document.querySelector('.chat-messages');
        if (messagesContainer) {
          messagesContainer.scrollTop = messagesContainer.scrollHeight;
        }
      }, 100);
    } catch (error) {
      console.error("Failed to send message:", error);
    } finally {
      messageForm.sending = false;
      vlens.scheduleRedraw();
    }
  };

  const formatTimestamp = (timestamp: Date) => {
    const now = new Date();
    const diff = now.getTime() - timestamp.getTime();
    const minutes = Math.floor(diff / 60000);
    const hours = Math.floor(diff / 3600000);
    const days = Math.floor(diff / 86400000);

    if (minutes < 1) return "Just now";
    if (minutes < 60) return `${minutes}m ago`;
    if (hours < 24) return `${hours}h ago`;
    if (days < 7) return `${days}d ago`;

    return timestamp.toLocaleDateString();
  };

  const getAvatarIcon = (senderName: string) => {
    // Simple avatar logic - in real app would use proper profile photos
    if (senderName === "Mom") return "👩";
    if (senderName === "Dad") return "👨";
    if (senderName === "Emma") return "👧";
    return "👤";
  };

  return (
    <div className="chat-page">
      <div className="chat-header">
        <h1>Family Chat</h1>
        <p>Stay connected with your family</p>
      </div>

      <div className="chat-content">
        <div className="chat-messages">
          {chatState.messages.map((msg) => (
            <div
              key={msg.id}
              className={`message ${msg.isCurrentUser ? "message-own" : "message-other"}`}
            >
              {!msg.isCurrentUser && (
                <div className="message-avatar">
                  <span className="avatar-icon">{getAvatarIcon(msg.senderName)}</span>
                </div>
              )}
              <div className="message-content">
                {!msg.isCurrentUser && (
                  <div className="message-sender">{msg.senderName}</div>
                )}
                <div className="message-bubble">
                  <p className="message-text">{msg.message}</p>
                  <span className="message-timestamp">
                    {formatTimestamp(msg.timestamp)}
                  </span>
                </div>
              </div>
            </div>
          ))}
        </div>

        <form className="chat-input-form" onSubmit={handleSendMessage}>
          <div className="input-container">
            <input
              type="text"
              placeholder="Type your message..."
              value={messageForm.message}
              onInput={(e) => {
                messageForm.message = (e.target as HTMLInputElement).value;
                vlens.scheduleRedraw();
              }}
              disabled={messageForm.sending}
              className="message-input"
              autoComplete="off"
            />
            <button
              type="submit"
              disabled={!messageForm.message.trim() || messageForm.sending}
              className="send-button"
            >
              {messageForm.sending ? "..." : "Send"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};