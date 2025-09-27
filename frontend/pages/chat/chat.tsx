import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { logInfo } from "../../lib/logger";
import "./chat-styles";

// Use ChatMessage type from server bindings

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

const useChatState = vlens.declareHook((): { messages: server.ChatMessage[] } => ({
  messages: [],
}));

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.GetChatMessagesResponse>({ messages: [] });
  }

  return server.GetChatMessages({ limit: null, offset: null });
}

export function view(
  route: string,
  prefix: string,
  data: server.GetChatMessagesResponse
): preact.ComponentChild {
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
  data: server.GetChatMessagesResponse;
}

const ChatPage = ({ user, data }: ChatPageProps) => {
  const messageForm = useMessageForm();
  const chatState = useChatState();

  // Initialize chat state with data from server
  if (data.messages && chatState.messages.length === 0) {
    chatState.messages = data.messages;
  }

  const handleSendMessage = async (e: Event) => {
    e.preventDefault();

    if (!messageForm.message.trim() || messageForm.sending) {
      return;
    }

    messageForm.sending = true;
    vlens.scheduleRedraw();

    try {
      // Send message to server
      const [result, error] = await server.SendMessage({
        content: messageForm.message.trim(),
      });

      if (result && !error) {
        // Add new message to local state
        chatState.messages = [...chatState.messages, result.message];
        messageForm.message = "";

        logInfo("ui", "Message sent", { messageId: result.message.id });
      } else {
        console.error("Failed to send message:", error);
      }

      // Scroll to bottom after sending
      setTimeout(() => {
        const messagesContainer = document.querySelector(".chat-messages");
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

  const formatTimestamp = (createdAt: string) => {
    const timestamp = new Date(createdAt);
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

  const getAvatarIcon = (userName: string) => {
    // Simple avatar logic based on name
    // Could be enhanced to use actual profile photos in the future
    const firstChar = userName.charAt(0).toUpperCase();
    return firstChar;
  };

  return (
    <div className="chat-page">
      <div className="chat-header">
        <h1>Family Chat</h1>
        <p>Stay connected with your family</p>
      </div>

      <div className="chat-content">
        <div className="chat-messages">
          {chatState.messages.map(msg => {
            const isCurrentUser = msg.userId === user.id;
            return (
              <div
                key={msg.id}
                className={`message ${isCurrentUser ? "message-own" : "message-other"}`}
              >
                {!isCurrentUser && (
                  <div className="message-avatar">
                    <span className="avatar-icon">{getAvatarIcon(msg.userName)}</span>
                  </div>
                )}
                <div className="message-content">
                  {!isCurrentUser && <div className="message-sender">{msg.userName}</div>}
                  <div className="message-bubble">
                    <p className="message-text">{msg.content}</p>
                    <span className="message-timestamp">{formatTimestamp(msg.createdAt)}</span>
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        <form className="chat-input-form" onSubmit={handleSendMessage}>
          <div className="input-container">
            <input
              type="text"
              placeholder="Type your message..."
              value={messageForm.message}
              onInput={e => {
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
