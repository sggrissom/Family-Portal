import * as preact from "preact";
import * as vlens from "vlens";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import { logInfo } from "../../lib/logger";
import {
  useChatWebsocket,
  connectWebSocket,
  disconnectWebSocket,
  destroyWebSocket,
  sendTypingIndicator,
  getConnectionStateText,
  getConnectionStateColor,
  isWebSocketSupported,
  type ConnectionState,
  type WebSocketEventHandlers,
} from "../../hooks/useChatWebsocket";
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

const useChatState = vlens.declareHook(
  (): {
    messages: server.ChatMessage[];
    initialized: boolean;
    sentClientMessageIds: Set<string>;
    lifecycleInitialized: boolean;
  } => ({
    messages: [],
    initialized: false,
    sentClientMessageIds: new Set<string>(),
    lifecycleInitialized: false,
  })
);

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
  const wsState = useChatWebsocket();

  // Initialize chat state with data from server (only once)
  if (data.messages && !chatState.initialized) {
    chatState.messages = data.messages;
    chatState.initialized = true;
  }

  // WebSocket event handlers
  const wsHandlers: WebSocketEventHandlers = {
    onNewMessage: (message: server.ChatMessage) => {
      // Skip messages that this tab sent (prevents race condition duplicates)
      if (message.clientMessageId && chatState.sentClientMessageIds.has(message.clientMessageId)) {
        return;
      }

      // Add message if not already present (supports multi-tab for same user)
      const exists = chatState.messages.some(m => m.id === message.id);
      if (!exists) {
        chatState.messages = [...chatState.messages, message];
        vlens.scheduleRedraw();

        // Scroll to bottom
        setTimeout(() => {
          const messagesContainer = document.querySelector(".chat-messages");
          if (messagesContainer) {
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
          }
        }, 100);
      }
    },

    onDeleteMessage: (messageId: number, userId: number) => {
      // Remove message from state
      const messageToDelete = chatState.messages.find(m => m.id === messageId);
      chatState.messages = chatState.messages.filter(m => m.id !== messageId);
      // Clean up tracking Set
      if (messageToDelete?.clientMessageId) {
        chatState.sentClientMessageIds.delete(messageToDelete.clientMessageId);
      }
      vlens.scheduleRedraw();
    },

    onUserTyping: (userId: number, userName: string, isTyping: boolean) => {
      // Handle typing indicators (could be implemented later)
      logInfo("ui", "User typing", { userId, userName, isTyping });
    },

    onUserOnline: (userId: number, userName: string) => {
      logInfo("ui", "User came online", { userId, userName });
    },

    onUserOffline: (userId: number, userName: string) => {
      logInfo("ui", "User went offline", { userId, userName });
    },

    onConnectionStateChange: (state: ConnectionState) => {
      vlens.scheduleRedraw();
    },

    onError: (error: string) => {
      logInfo("ui", "WebSocket error", { error });
    },
  };

  // Connect websocket on component initialization
  if (
    isWebSocketSupported() &&
    wsState.connectionState === "disconnected" &&
    !wsState.isDestroyed
  ) {
    connectWebSocket(wsState, wsHandlers);
  }

  // Set up lifecycle management once per component instance
  if (wsState.socket && !chatState.lifecycleInitialized && window.addEventListener) {
    chatState.lifecycleInitialized = true;

    const handleBeforeUnload = () => {
      destroyWebSocket(wsState);
    };

    // Monitor route changes to destroy WebSocket when leaving chat page
    const handleRouteChange = () => {
      // Check if current route is still /chat
      if (!window.location.pathname.startsWith("/chat")) {
        destroyWebSocket(wsState);
        // Clean up listeners
        cleanup();
      }
    };

    // More aggressive cleanup - check periodically if we're still on chat page
    const routeChecker = setInterval(() => {
      if (!window.location.pathname.startsWith("/chat") && !wsState.isDestroyed) {
        destroyWebSocket(wsState);
        cleanup();
      }
    }, 1000);

    const cleanup = () => {
      window.removeEventListener("beforeunload", handleBeforeUnload);
      window.removeEventListener("popstate", handleRouteChange);
      window.removeEventListener("hashchange", handleRouteChange);
      clearInterval(routeChecker);
      chatState.lifecycleInitialized = false;
      delete (window as any).chatWebSocketCleanup;
    };

    // Listen to multiple navigation events
    window.addEventListener("beforeunload", handleBeforeUnload);
    window.addEventListener("popstate", handleRouteChange);
    window.addEventListener("hashchange", handleRouteChange);

    // Store cleanup function for manual cleanup if needed
    (window as any).chatWebSocketCleanup = cleanup;

    // Check route immediately in case of direct navigation
    if (!window.location.pathname.startsWith("/chat")) {
      setTimeout(() => {
        if (!window.location.pathname.startsWith("/chat")) {
          destroyWebSocket(wsState);
          cleanup();
        }
      }, 100);
    }
  }

  const handleSendMessage = async (e: Event) => {
    e.preventDefault();

    if (!messageForm.message.trim() || messageForm.sending) {
      return;
    }

    const messageContent = messageForm.message.trim();
    // Generate unique client message ID
    const clientMessageId = `${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;

    messageForm.sending = true;
    messageForm.message = ""; // Clear input immediately

    // Track that this tab sent this message (prevents WebSocket duplicate)
    chatState.sentClientMessageIds.add(clientMessageId);

    // Create optimistic message for immediate UI feedback
    const optimisticMessage: server.ChatMessage = {
      id: -Date.now(), // Negative ID to indicate pending
      familyId: user.familyId,
      userId: user.id,
      userName: user.name,
      content: messageContent,
      createdAt: new Date().toISOString(),
      clientMessageId: clientMessageId,
    };

    // Add optimistic message to UI immediately
    chatState.messages = [...chatState.messages, optimisticMessage];
    vlens.scheduleRedraw();

    // Scroll to bottom immediately
    setTimeout(() => {
      const messagesContainer = document.querySelector(".chat-messages");
      if (messagesContainer) {
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
      }
    }, 50);

    try {
      // Send message to server
      const [result, error] = await server.SendMessage({
        content: messageContent,
        clientMessageId: clientMessageId,
      });

      if (result && !error) {
        // Replace optimistic message with real message from server
        chatState.messages = chatState.messages.map(msg =>
          msg.id === optimisticMessage.id ? result.message : msg
        );
      } else {
        // Remove optimistic message on error
        chatState.messages = chatState.messages.filter(msg => msg.id !== optimisticMessage.id);
        // Remove from tracking set on error
        chatState.sentClientMessageIds.delete(clientMessageId);

        // Restore message to input for retry
        messageForm.message = messageContent;

        console.error("Failed to send message:", error);
      }
    } catch (error) {
      // Remove optimistic message on error
      chatState.messages = chatState.messages.filter(msg => msg.id !== optimisticMessage.id);
      // Remove from tracking set on error
      chatState.sentClientMessageIds.delete(clientMessageId);

      // Restore message to input for retry
      messageForm.message = messageContent;

      console.error("Failed to send message:", error);
    } finally {
      messageForm.sending = false;
      vlens.scheduleRedraw();
    }
  };

  const handleDeleteMessage = async (messageId: number) => {
    try {
      const [result, error] = await server.DeleteMessage({
        id: messageId,
      });

      if (result && !error && result.success) {
        // Remove message from UI immediately (WebSocket will also handle this)
        const messageToDelete = chatState.messages.find(msg => msg.id === messageId);
        chatState.messages = chatState.messages.filter(msg => msg.id !== messageId);
        // Clean up tracking Set
        if (messageToDelete?.clientMessageId) {
          chatState.sentClientMessageIds.delete(messageToDelete.clientMessageId);
        }
        vlens.scheduleRedraw();
      } else {
        console.error("Failed to delete message:", error);
      }
    } catch (error) {
      console.error("Failed to delete message:", error);
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
        <div className="chat-header-content">
          <h1>Family Chat</h1>
          <p>Stay connected with your family</p>
        </div>
        {isWebSocketSupported() && (
          <div className="connection-status">
            <div
              className="connection-indicator"
              style={{
                backgroundColor: getConnectionStateColor(wsState.connectionState),
              }}
            ></div>
            <span className="connection-text">
              {getConnectionStateText(wsState.connectionState)}
              {wsState.connectionState === "reconnecting" &&
                ` (${wsState.reconnectAttempts}/${wsState.maxReconnectAttempts})`}
            </span>
          </div>
        )}
        {!isWebSocketSupported() && (
          <div className="connection-status">
            <div className="connection-indicator" style={{ backgroundColor: "#9ca3af" }}></div>
            <span className="connection-text">Real-time not supported</span>
          </div>
        )}
      </div>

      <div className="chat-content">
        <div className="chat-messages">
          {chatState.messages.map(msg => {
            const isCurrentUser = msg.userId === user.id;
            const isPending = msg.id < 0; // Negative IDs indicate pending messages
            return (
              <div
                key={msg.id}
                className={`message ${isCurrentUser ? "message-own" : "message-other"} ${
                  isPending ? "message-pending" : ""
                }`}
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
                    <div className="message-footer">
                      <span className="message-timestamp">{formatTimestamp(msg.createdAt)}</span>
                      {isPending && <span className="message-status">Sending...</span>}
                      {isCurrentUser && !isPending && (
                        <button
                          className="delete-message-btn"
                          onClick={() => handleDeleteMessage(msg.id)}
                          title="Delete message"
                        >
                          ×
                        </button>
                      )}
                    </div>
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
