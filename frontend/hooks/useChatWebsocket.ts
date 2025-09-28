import * as vlens from "vlens";
import * as server from "../server";
import { logInfo, logWarn, logError } from "../lib/logger";

// WebSocket message types (matching backend)
export const WS_MSG_TYPES = {
  NEW_MESSAGE: "new_message",
  DELETE_MESSAGE: "delete_message",
  USER_TYPING: "user_typing",
  USER_ONLINE: "user_online",
  USER_OFFLINE: "user_offline",
  HEARTBEAT: "heartbeat",
  ERROR: "error",
} as const;

// Connection states
export type ConnectionState =
  | "disconnected"
  | "connecting"
  | "connected"
  | "reconnecting"
  | "error"
  | "failed";

// WebSocket message structure
export interface WSMessage {
  type: string;
  payload: any;
  timestamp: string;
}

// Message payloads
export interface WSNewMessagePayload {
  message: server.ChatMessage;
}

export interface WSDeleteMessagePayload {
  messageId: number;
  userId: number;
}

export interface WSTypingPayload {
  userId: number;
  userName: string;
  isTyping: boolean;
}

export interface WSUserStatusPayload {
  userId: number;
  userName: string;
  isOnline: boolean;
}

// Queued message for offline sending
export interface QueuedMessage {
  id: string;
  type: string;
  payload: any;
  timestamp: number;
  retries: number;
}

// Event handlers
export interface WebSocketEventHandlers {
  onNewMessage?: (message: server.ChatMessage) => void;
  onDeleteMessage?: (messageId: number, userId: number) => void;
  onUserTyping?: (userId: number, userName: string, isTyping: boolean) => void;
  onUserOnline?: (userId: number, userName: string) => void;
  onUserOffline?: (userId: number, userName: string) => void;
  onConnectionStateChange?: (state: ConnectionState) => void;
  onError?: (error: string) => void;
}

// WebSocket state
export interface WebSocketState {
  socket: WebSocket | null;
  connectionState: ConnectionState;
  reconnectAttempts: number;
  maxReconnectAttempts: number;
  reconnectDelay: number;
  maxReconnectDelay: number;
  messageQueue: QueuedMessage[];
  lastHeartbeat: number;
  heartbeatInterval: number | null;
  isDestroyed: boolean;
  eventHandlers: WebSocketEventHandlers;
  supportsFallback: boolean;
  autoReconnect: boolean;
  authToken: string | null;
  reconnectTimeout: number | null;
  lastActivityTime: number;
  watchdogInterval: number | null;
  watchdogTimeout: number;
}

// Create the hook
export const useChatWebsocket = vlens.declareHook(
  (): WebSocketState => ({
    socket: null,
    connectionState: "disconnected",
    reconnectAttempts: 0,
    maxReconnectAttempts: 10,
    reconnectDelay: 1000, // Start with 1 second
    maxReconnectDelay: 30000, // Max 30 seconds
    messageQueue: [],
    lastHeartbeat: 0,
    heartbeatInterval: null,
    isDestroyed: false,
    eventHandlers: {},
    supportsFallback: true,
    autoReconnect: true,
    authToken: null,
    reconnectTimeout: null,
    lastActivityTime: 0,
    watchdogInterval: null,
    watchdogTimeout: 90000, // 90 seconds
  })
);

// Connect to WebSocket
export function connectWebSocket(
  state: WebSocketState,
  handlers: WebSocketEventHandlers = {}
): void {
  if (
    state.isDestroyed ||
    state.connectionState === "connecting" ||
    state.connectionState === "connected"
  ) {
    return;
  }

  // Store event handlers
  state.eventHandlers = handlers;
  setConnectionState(state, "connecting");

  try {
    // Determine WebSocket URL (no token needed - cookies sent automatically)
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/ws/chat`;

    // Create WebSocket connection
    const socket = new WebSocket(wsUrl);
    state.socket = socket;

    // Connection opened
    socket.onopen = () => {
      logInfo("ui", "WebSocket connected");
      setConnectionState(state, "connected");
      state.reconnectAttempts = 0;
      state.reconnectDelay = 1000; // Reset delay
      state.lastActivityTime = Date.now();

      // Start heartbeat and watchdog
      startHeartbeat(state);
      startWatchdog(state);

      // Process queued messages
      processMessageQueue(state);

      vlens.scheduleRedraw();
    };

    // Message received
    socket.onmessage = event => {
      try {
        const wsMessage: WSMessage = JSON.parse(event.data);
        handleIncomingMessage(state, wsMessage);

        // Update activity and heartbeat time
        state.lastHeartbeat = Date.now();
        state.lastActivityTime = Date.now();
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : String(error);
        logError("ui", "Failed to parse WebSocket message", { error: errorMessage });
      }
    };

    // Connection closed
    socket.onclose = event => {
      logInfo("ui", "WebSocket connection closed", {
        code: event.code,
        reason: event.reason,
        wasClean: event.wasClean,
      });

      cleanup(state);

      // Attempt reconnection if not destroyed and auto-reconnect is enabled
      if (!state.isDestroyed && state.autoReconnect && event.code !== 1000) {
        attemptReconnect(state);
      } else {
        setConnectionState(state, "disconnected");
      }

      vlens.scheduleRedraw();
    };

    // Connection error
    socket.onerror = error => {
      logError("ui", "WebSocket error", { error });

      if (state.connectionState === "connecting") {
        setConnectionState(state, "error");
        attemptReconnect(state);
      }

      vlens.scheduleRedraw();
    };
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    logError("ui", "Failed to create WebSocket connection", { error: errorMessage });
    setConnectionState(state, "error");
    attemptReconnect(state);
  }
}

// Disconnect WebSocket
export function disconnectWebSocket(state: WebSocketState): void {
  state.autoReconnect = false;

  if (state.socket) {
    state.socket.close(1000, "User disconnect");
  }

  cleanup(state);
  setConnectionState(state, "disconnected");
  vlens.scheduleRedraw();
}

// Destroy WebSocket (cleanup on component unmount)
export function destroyWebSocket(state: WebSocketState): void {
  state.isDestroyed = true;
  state.autoReconnect = false;

  // Clear any pending reconnection attempts
  clearAllTimeouts();

  if (state.socket) {
    state.socket.close(1000, "Component unmount");
  }

  cleanup(state);
  vlens.scheduleRedraw();
}

// Send message through WebSocket
export function sendWebSocketMessage(state: WebSocketState, type: string, payload: any): void {
  const message: QueuedMessage = {
    id: generateMessageId(),
    type,
    payload,
    timestamp: Date.now(),
    retries: 0,
  };

  if (state.connectionState === "connected" && state.socket) {
    try {
      const wsMessage: WSMessage = {
        type: message.type,
        payload: message.payload,
        timestamp: new Date().toISOString(),
      };

      state.socket.send(JSON.stringify(wsMessage));
      logInfo("ui", "WebSocket message sent", { type, messageId: message.id });
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      logError("ui", "Failed to send WebSocket message", { error: errorMessage });
      queueMessage(state, message);
    }
  } else {
    // Queue message for later sending
    queueMessage(state, message);
  }
}

// Send typing indicator
export function sendTypingIndicator(state: WebSocketState, isTyping: boolean): void {
  if (state.connectionState === "connected") {
    sendWebSocketMessage(state, WS_MSG_TYPES.USER_TYPING, { isTyping });
  }
}

// Private helper functions

// Global timeout tracking for aggressive cleanup
let allTimeouts: Set<number> = new Set();

function trackTimeout(timeoutId: number): void {
  allTimeouts.add(timeoutId);
}

function clearAllTimeouts(): void {
  allTimeouts.forEach(id => clearTimeout(id));
  allTimeouts.clear();
}

function setConnectionState(state: WebSocketState, newState: ConnectionState): void {
  if (state.connectionState !== newState) {
    state.connectionState = newState;

    if (state.eventHandlers.onConnectionStateChange) {
      state.eventHandlers.onConnectionStateChange(newState);
    }

    logInfo("ui", "WebSocket connection state changed", { state: newState });
  }
}

function handleIncomingMessage(state: WebSocketState, wsMessage: WSMessage): void {
  const { type, payload } = wsMessage;

  switch (type) {
    case WS_MSG_TYPES.NEW_MESSAGE:
      if (state.eventHandlers.onNewMessage) {
        const data = payload as WSNewMessagePayload;
        state.eventHandlers.onNewMessage(data.message);
      }
      break;

    case WS_MSG_TYPES.DELETE_MESSAGE:
      if (state.eventHandlers.onDeleteMessage) {
        const data = payload as WSDeleteMessagePayload;
        state.eventHandlers.onDeleteMessage(data.messageId, data.userId);
      }
      break;

    case WS_MSG_TYPES.USER_TYPING:
      if (state.eventHandlers.onUserTyping) {
        const data = payload as WSTypingPayload;
        state.eventHandlers.onUserTyping(data.userId, data.userName, data.isTyping);
      }
      break;

    case WS_MSG_TYPES.USER_ONLINE:
      if (state.eventHandlers.onUserOnline) {
        const data = payload as WSUserStatusPayload;
        state.eventHandlers.onUserOnline(data.userId, data.userName);
      }
      break;

    case WS_MSG_TYPES.USER_OFFLINE:
      if (state.eventHandlers.onUserOffline) {
        const data = payload as WSUserStatusPayload;
        state.eventHandlers.onUserOffline(data.userId, data.userName);
      }
      break;

    case WS_MSG_TYPES.HEARTBEAT:
      // Heartbeat response, no action needed
      break;

    case WS_MSG_TYPES.ERROR:
      if (state.eventHandlers.onError) {
        state.eventHandlers.onError(payload.message || "Unknown WebSocket error");
      }
      break;

    default:
      logWarn("ui", "Unknown WebSocket message type", { type, payload });
  }
}

function attemptReconnect(state: WebSocketState): void {
  if (
    state.isDestroyed ||
    !state.autoReconnect ||
    state.reconnectAttempts >= state.maxReconnectAttempts
  ) {
    setConnectionState(state, "failed");
    logError("ui", "WebSocket reconnection failed - max attempts reached", {
      attempts: state.reconnectAttempts,
      maxAttempts: state.maxReconnectAttempts,
    });
    return;
  }

  state.reconnectAttempts++;
  setConnectionState(state, "reconnecting");

  const delay = Math.min(
    state.reconnectDelay * Math.pow(2, state.reconnectAttempts - 1),
    state.maxReconnectDelay
  );

  logInfo("ui", "WebSocket reconnecting", {
    attempt: state.reconnectAttempts,
    delay,
  });

  const timeoutId = window.setTimeout(() => {
    if (!state.isDestroyed && state.autoReconnect) {
      connectWebSocket(state, state.eventHandlers);
    }
    state.reconnectTimeout = null;
    allTimeouts.delete(timeoutId);
  }, delay);

  state.reconnectTimeout = timeoutId;
  trackTimeout(timeoutId);
}

function startHeartbeat(state: WebSocketState): void {
  if (state.heartbeatInterval) {
    clearInterval(state.heartbeatInterval);
  }

  state.heartbeatInterval = window.setInterval(() => {
    if (state.socket && state.socket.readyState === WebSocket.OPEN) {
      sendWebSocketMessage(state, WS_MSG_TYPES.HEARTBEAT, "ping");
    }
  }, 30000); // Send heartbeat every 30 seconds
}

function startWatchdog(state: WebSocketState): void {
  if (state.watchdogInterval) {
    clearInterval(state.watchdogInterval);
  }

  state.watchdogInterval = window.setInterval(() => {
    if (state.isDestroyed || !state.socket) {
      return;
    }

    const now = Date.now();
    const timeSinceLastActivity = now - state.lastActivityTime;

    if (timeSinceLastActivity > state.watchdogTimeout) {
      logWarn("ui", "WebSocket watchdog timeout - forcing reconnect", {
        timeSinceLastActivity,
        watchdogTimeout: state.watchdogTimeout,
        connectionState: state.connectionState,
      });

      // Force close the connection to trigger reconnect
      if (state.socket && state.socket.readyState === WebSocket.OPEN) {
        state.socket.close(1000, "Watchdog timeout");
      }
    }
  }, 30000); // Check every 30 seconds
}

function cleanup(state: WebSocketState): void {
  if (state.heartbeatInterval) {
    clearInterval(state.heartbeatInterval);
    state.heartbeatInterval = null;
  }

  if (state.watchdogInterval) {
    clearInterval(state.watchdogInterval);
    state.watchdogInterval = null;
  }

  if (state.reconnectTimeout) {
    clearTimeout(state.reconnectTimeout);
    state.reconnectTimeout = null;
  }

  state.socket = null;
  state.lastHeartbeat = 0;
  state.lastActivityTime = 0;
}

function queueMessage(state: WebSocketState, message: QueuedMessage): void {
  // Limit queue size to prevent memory issues
  if (state.messageQueue.length > 100) {
    state.messageQueue.shift(); // Remove oldest message
  }

  state.messageQueue.push(message);
  logInfo("ui", "WebSocket message queued", {
    messageId: message.id,
    queueSize: state.messageQueue.length,
  });
}

function processMessageQueue(state: WebSocketState): void {
  if (!state.socket || state.socket.readyState !== WebSocket.OPEN) {
    return;
  }

  const toProcess = [...state.messageQueue];
  state.messageQueue = [];

  for (const message of toProcess) {
    try {
      const wsMessage: WSMessage = {
        type: message.type,
        payload: message.payload,
        timestamp: new Date().toISOString(),
      };

      state.socket.send(JSON.stringify(wsMessage));
      logInfo("ui", "Queued WebSocket message sent", { messageId: message.id });
    } catch (error) {
      logError("ui", "Failed to send queued message", {
        messageId: message.id,
        error: error instanceof Error ? error.message : String(error),
      });

      // Re-queue with retry limit
      if (message.retries < 3) {
        message.retries++;
        queueMessage(state, message);
      }
    }
  }
}

function generateMessageId(): string {
  return `${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;
}

// Utility function to check WebSocket support
export function isWebSocketSupported(): boolean {
  return "WebSocket" in window && window.WebSocket !== undefined;
}

// Get connection state display text
export function getConnectionStateText(state: ConnectionState): string {
  switch (state) {
    case "connected":
      return "Connected";
    case "connecting":
      return "Connecting...";
    case "reconnecting":
      return "Reconnecting...";
    case "disconnected":
      return "Disconnected";
    case "error":
      return "Connection Error";
    case "failed":
      return "Connection Failed";
    default:
      return "Unknown";
  }
}

// Get connection state color for UI
export function getConnectionStateColor(state: ConnectionState): string {
  switch (state) {
    case "connected":
      return "#4ade80"; // green
    case "connecting":
    case "reconnecting":
      return "#fbbf24"; // yellow
    case "disconnected":
      return "#9ca3af"; // gray
    case "error":
    case "failed":
      return "#ef4444"; // red
    default:
      return "#9ca3af"; // gray
  }
}
