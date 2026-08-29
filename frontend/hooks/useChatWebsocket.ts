import * as vlens from "vlens";
import * as server from "../server";
import { logInfo, logWarn, logError } from "../lib/logger";

export const WS_MSG_TYPES = {
  NEW_MESSAGE: "new_message",
  DELETE_MESSAGE: "delete_message",
  USER_TYPING: "user_typing",
  USER_ONLINE: "user_online",
  USER_OFFLINE: "user_offline",
  HEARTBEAT: "heartbeat",
  ERROR: "error",
} as const;

export type ConnectionState =
  | "disconnected"
  | "connecting"
  | "connected"
  | "reconnecting"
  | "error"
  | "failed";

export interface WSMessage {
  type: string;
  payload: any;
  timestamp: string;
}

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

export interface QueuedMessage {
  id: string;
  type: string;
  payload: any;
  timestamp: number;
  retries: number;
}

export interface WebSocketEventHandlers {
  onNewMessage?: (message: server.ChatMessage) => void;
  onDeleteMessage?: (messageId: number, userId: number) => void;
  onUserTyping?: (userId: number, userName: string, isTyping: boolean) => void;
  onUserOnline?: (userId: number, userName: string) => void;
  onUserOffline?: (userId: number, userName: string) => void;
  onConnectionStateChange?: (state: ConnectionState) => void;
  onError?: (error: string) => void;
}

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

export const useChatWebsocket = vlens.declareHook(
  (): WebSocketState => ({
    socket: null,
    connectionState: "disconnected",
    reconnectAttempts: 0,
    maxReconnectAttempts: 10,
    reconnectDelay: 1000,
    maxReconnectDelay: 30000,
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
    watchdogTimeout: 90000,
  })
);

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

  state.eventHandlers = handlers;
  setConnectionState(state, "connecting");

  try {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/ws/chat`;

    const socket = new WebSocket(wsUrl);
    state.socket = socket;

    socket.onopen = () => {
      setConnectionState(state, "connected");
      state.reconnectAttempts = 0;
      state.reconnectDelay = 1000;
      state.lastActivityTime = Date.now();

      startHeartbeat(state);
      startWatchdog(state);

      processMessageQueue(state);

      vlens.scheduleRedraw();
    };

    socket.onmessage = event => {
      try {
        const wsMessage: WSMessage = JSON.parse(event.data);
        handleIncomingMessage(state, wsMessage);

        state.lastHeartbeat = Date.now();
        state.lastActivityTime = Date.now();
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : String(error);
        logError("ui", "Failed to parse WebSocket message", { error: errorMessage });
      }
    };

    socket.onclose = event => {
      if (event.code !== 1000) {
        logInfo("ui", "WebSocket connection closed", {
          code: event.code,
        });
      }

      cleanup(state);

      if (!state.isDestroyed && state.autoReconnect && event.code !== 1000) {
        attemptReconnect(state);
      } else {
        setConnectionState(state, "disconnected");
      }

      vlens.scheduleRedraw();
    };

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

export function disconnectWebSocket(state: WebSocketState): void {
  state.autoReconnect = false;

  if (state.socket) {
    state.socket.close(1000, "User disconnect");
  }

  cleanup(state);
  setConnectionState(state, "disconnected");
  vlens.scheduleRedraw();
}

export function destroyWebSocket(state: WebSocketState): void {
  state.isDestroyed = true;
  state.autoReconnect = false;

  clearAllTimeouts();

  if (state.socket) {
    state.socket.close(1000, "Component unmount");
  }

  cleanup(state);
  vlens.scheduleRedraw();
}

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
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      logError("ui", "Failed to send WebSocket message", { error: errorMessage });
      queueMessage(state, message);
    }
  } else {
    queueMessage(state, message);
  }
}

export function sendTypingIndicator(state: WebSocketState, isTyping: boolean): void {
  if (state.connectionState === "connected") {
    sendWebSocketMessage(state, WS_MSG_TYPES.USER_TYPING, { isTyping });
  }
}

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
  }, 30000);
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

      if (state.socket && state.socket.readyState === WebSocket.OPEN) {
        state.socket.close(1000, "Watchdog timeout");
      }
    }
  }, 30000);
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
  if (state.messageQueue.length > 100) {
    state.messageQueue.shift();
  }

  state.messageQueue.push(message);
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
    } catch (error) {
      logError("ui", "Failed to send queued message", {
        messageId: message.id,
        error: error instanceof Error ? error.message : String(error),
      });

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

export function isWebSocketSupported(): boolean {
  return "WebSocket" in window && window.WebSocket !== undefined;
}

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

export function getConnectionStateColor(state: ConnectionState): string {
  switch (state) {
    case "connected":
      return "#4ade80";
    case "connecting":
    case "reconnecting":
      return "#fbbf24";
    case "disconnected":
      return "#9ca3af";
    case "error":
    case "failed":
      return "#ef4444";
    default:
      return "#9ca3af";
  }
}
