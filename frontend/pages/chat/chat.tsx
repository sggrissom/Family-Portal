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
    expandedThreadKeys: Set<string>;
  } => ({
    messages: [],
    initialized: false,
    sentClientMessageIds: new Set<string>(),
    lifecycleInitialized: false,
    expandedThreadKeys: new Set<string>(),
  })
);

export async function fetch(route: string, prefix: string) {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok<server.GetChatMessagesResponse>({ messages: [] });
  }

  return server.GetChatMessages({ limit: null, offset: null, familyId: 0 });
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
  user: any;
  data: server.GetChatMessagesResponse;
}

const ChatPage = ({ user, data }: ChatPageProps) => {
  const messageForm = useMessageForm();
  const chatState = useChatState();
  const wsState = useChatWebsocket();

  if (data.messages && !chatState.initialized) {
    chatState.messages = data.messages;
    chatState.initialized = true;
  }

  const wsHandlers: WebSocketEventHandlers = {
    onNewMessage: (message: server.ChatMessage) => {
      if (message.clientMessageId && chatState.sentClientMessageIds.has(message.clientMessageId)) {
        return;
      }

      const exists = chatState.messages.some(m => m.id === message.id);
      if (!exists) {
        chatState.messages = [message, ...chatState.messages];
        vlens.scheduleRedraw();

        setTimeout(() => {
          const messagesContainer = document.querySelector(".chat-messages");
          if (messagesContainer) {
            messagesContainer.scrollTop = 0;
          }
        }, 100);
      }
    },

    onDeleteMessage: (messageId: number, userId: number) => {
      const messageToDelete = chatState.messages.find(m => m.id === messageId);
      chatState.messages = chatState.messages.filter(m => m.id !== messageId);
      if (messageToDelete?.clientMessageId) {
        chatState.sentClientMessageIds.delete(messageToDelete.clientMessageId);
      }
      vlens.scheduleRedraw();
    },

    onUserTyping: (userId: number, userName: string, isTyping: boolean) => {
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

  if (
    isWebSocketSupported() &&
    wsState.connectionState === "disconnected" &&
    !wsState.isDestroyed
  ) {
    connectWebSocket(wsState, wsHandlers);
  }

  if (wsState.socket && !chatState.lifecycleInitialized && window.addEventListener) {
    chatState.lifecycleInitialized = true;

    const handleBeforeUnload = () => {
      destroyWebSocket(wsState);
    };

    const handleRouteChange = () => {
      if (!window.location.pathname.startsWith("/chat")) {
        destroyWebSocket(wsState);
        cleanup();
      }
    };

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

    window.addEventListener("beforeunload", handleBeforeUnload);
    window.addEventListener("popstate", handleRouteChange);
    window.addEventListener("hashchange", handleRouteChange);

    (window as any).chatWebSocketCleanup = cleanup;

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
    const clientMessageId = `${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;

    messageForm.sending = true;
    messageForm.message = "";

    chatState.sentClientMessageIds.add(clientMessageId);

    const optimisticMessage: server.ChatMessage = {
      id: -Date.now(),
      familyId: user.familyId,
      userId: user.id,
      userName: user.name,
      content: messageContent,
      createdAt: new Date().toISOString(),
      clientMessageId: clientMessageId,
    };

    chatState.messages = [optimisticMessage, ...chatState.messages];
    vlens.scheduleRedraw();

    setTimeout(() => {
      const messagesContainer = document.querySelector(".chat-messages");
      if (messagesContainer) {
        messagesContainer.scrollTop = 0;
      }
    }, 50);

    try {
      const [result, error] = await server.SendMessage({
        content: messageContent,
        clientMessageId: clientMessageId,
        familyId: 0,
      });

      if (result && !error) {
        chatState.messages = chatState.messages.map(msg =>
          msg.id === optimisticMessage.id ? result.message : msg
        );
      } else {
        chatState.messages = chatState.messages.filter(msg => msg.id !== optimisticMessage.id);
        chatState.sentClientMessageIds.delete(clientMessageId);

        messageForm.message = messageContent;

        console.error("Failed to send message:", error);
      }
    } catch (error) {
      chatState.messages = chatState.messages.filter(msg => msg.id !== optimisticMessage.id);
      chatState.sentClientMessageIds.delete(clientMessageId);

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
        const messageToDelete = chatState.messages.find(msg => msg.id === messageId);
        chatState.messages = chatState.messages.filter(msg => msg.id !== messageId);
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

  const getDayKey = (createdAt: string) => {
    const timestamp = new Date(createdAt);
    return [timestamp.getFullYear(), timestamp.getMonth(), timestamp.getDate()].join("-");
  };

  const getTodayKey = () => {
    const today = new Date();
    return [today.getFullYear(), today.getMonth(), today.getDate()].join("-");
  };

  const formatDateDividerLabel = (createdAt: string) => {
    const timestamp = new Date(createdAt);
    const today = new Date();
    const startOfToday = new Date(today.getFullYear(), today.getMonth(), today.getDate());
    const startOfTimestamp = new Date(
      timestamp.getFullYear(),
      timestamp.getMonth(),
      timestamp.getDate()
    );
    const diffDays = Math.round((startOfToday.getTime() - startOfTimestamp.getTime()) / 86400000);

    if (diffDays === 0) {
      return "Today";
    }
    if (diffDays === 1) {
      return "Yesterday";
    }

    const options: Intl.DateTimeFormatOptions = {
      weekday: "short",
      month: "short",
      day: "numeric",
    };
    if (timestamp.getFullYear() !== today.getFullYear()) {
      options.year = "numeric";
    }

    return timestamp.toLocaleDateString(undefined, options);
  };

  const formatThreadRange = (messages: server.ChatMessage[]) => {
    if (messages.length === 0) {
      return "";
    }

    const timestamps = messages
      .map(message => new Date(message.createdAt))
      .sort((a, b) => a.getTime() - b.getTime());
    const first = timestamps[0];
    const last = timestamps[timestamps.length - 1];

    return `${first.toLocaleTimeString(undefined, {
      hour: "numeric",
      minute: "2-digit",
    })} – ${last.toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" })}`;
  };

  const getAvatarIcon = (userName: string) => {
    const firstChar = userName.charAt(0).toUpperCase();
    return firstChar;
  };

  const renderMessage = (msg: server.ChatMessage) => {
    const isCurrentUser = msg.userId === user.id;
    const isPending = msg.id < 0;
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
                  aria-label="Delete message"
                >
                  ×
                </button>
              )}
            </div>
          </div>
        </div>
      </div>
    );
  };

  const todayKey = getTodayKey();
  const newestFirstMessages = [...chatState.messages].sort(
    (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
  );
  const threads = newestFirstMessages.reduce<
    { key: string; label: string; messages: server.ChatMessage[] }[]
  >((acc, msg) => {
    const dayKey = getDayKey(msg.createdAt);
    const currentThread = acc[acc.length - 1];

    if (!currentThread || currentThread.key !== dayKey) {
      acc.push({
        key: dayKey,
        label: formatDateDividerLabel(msg.createdAt),
        messages: [msg],
      });
      return acc;
    }

    currentThread.messages.push(msg);
    return acc;
  }, []);

  const renderedMessages = threads.map(thread => {
    const isCurrentDay = thread.key === todayKey;
    const isExpanded = isCurrentDay || chatState.expandedThreadKeys.has(thread.key);
    const threadClasses = [
      "chat-thread",
      isExpanded ? "chat-thread-expanded" : "chat-thread-collapsed",
    ]
      .filter(Boolean)
      .join(" ");

    return (
      <section key={`thread-${thread.key}`} className={threadClasses}>
        <button
          type="button"
          className="chat-thread-header"
          onClick={() => {
            if (isCurrentDay) {
              return;
            }

            const nextExpandedKeys = new Set(chatState.expandedThreadKeys);
            if (nextExpandedKeys.has(thread.key)) {
              nextExpandedKeys.delete(thread.key);
            } else {
              nextExpandedKeys.add(thread.key);
            }
            chatState.expandedThreadKeys = nextExpandedKeys;
            vlens.scheduleRedraw();
          }}
          aria-expanded={isExpanded}
        >
          <span className="chat-thread-title">{thread.label}</span>
          <span className="chat-thread-meta">
            {thread.messages.length} {thread.messages.length === 1 ? "message" : "messages"}
            {thread.messages.length > 1 && ` · ${formatThreadRange(thread.messages)}`}
          </span>
          {!isCurrentDay && (
            <span className="chat-thread-toggle">{isExpanded ? "Collapse" : "Open"}</span>
          )}
        </button>
        {isExpanded && (
          <div className="chat-thread-messages">{thread.messages.map(renderMessage)}</div>
        )}
      </section>
    );
  });

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
        <div className="chat-messages">{renderedMessages}</div>

        <form className="chat-input-form" onSubmit={handleSendMessage}>
          <div className="input-container">
            <input
              type="text"
              aria-label="Message"
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
