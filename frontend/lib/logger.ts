/**
 * Frontend logging utility
 * Provides structured logging that can be extended to send logs to backend
 */

type LogLevel = "debug" | "info" | "warn" | "error";
type LogCategory = "auth" | "photo" | "admin" | "api" | "ui" | "system";

interface LogEntry {
  timestamp: string;
  level: LogLevel;
  category: LogCategory;
  message: string;
  data?: any;
  userAgent?: string;
  url?: string;
}

class FrontendLogger {
  private isDevelopment: boolean;

  constructor() {
    this.isDevelopment =
      typeof window !== "undefined" &&
      (window.location.hostname === "localhost" || window.location.hostname === "127.0.0.1");
  }

  private log(level: LogLevel, category: LogCategory, message: string, data?: any): void {
    const entry: LogEntry = {
      timestamp: new Date().toISOString(),
      level,
      category,
      message,
      data,
    };

    if (typeof window !== "undefined") {
      entry.userAgent = window.navigator.userAgent;
      entry.url = window.location.href;
    }

    // For now, use console logging with structured format
    const prefix = `[${entry.timestamp}] [${level.toUpperCase()}] [${category.toUpperCase()}]`;

    switch (level) {
      case "debug":
        if (this.isDevelopment) {
          console.debug(prefix, message, data);
        }
        break;
      case "info":
        console.info(prefix, message, data);
        break;
      case "warn":
        console.warn(prefix, message, data);
        break;
      case "error":
        console.error(prefix, message, data);
        break;
    }

    // In the future, we could send critical errors to the backend:
    // if (level === 'error') {
    //   this.sendToBackend(entry);
    // }
  }

  // Public logging methods
  debug(category: LogCategory, message: string, data?: any): void {
    this.log("debug", category, message, data);
  }

  info(category: LogCategory, message: string, data?: any): void {
    this.log("info", category, message, data);
  }

  warn(category: LogCategory, message: string, data?: any): void {
    this.log("warn", category, message, data);
  }

  error(category: LogCategory, message: string, data?: any): void {
    this.log("error", category, message, data);
  }

  // Convenience methods for common scenarios
  authError(message: string, data?: any): void {
    this.error("auth", message, data);
  }

  apiError(message: string, data?: any): void {
    this.error("api", message, data);
  }

  photoError(message: string, data?: any): void {
    this.error("photo", message, data);
  }

  adminWarn(message: string, data?: any): void {
    this.warn("admin", message, data);
  }

  uiError(message: string, data?: any): void {
    this.error("ui", message, data);
  }

  // Future: Send logs to backend
  // private async sendToBackend(entry: LogEntry): Promise<void> {
  //   try {
  //     await fetch('/api/frontend-logs', {
  //       method: 'POST',
  //       headers: { 'Content-Type': 'application/json' },
  //       body: JSON.stringify(entry),
  //       credentials: 'include'
  //     });
  //   } catch (error) {
  //     // Fallback to console if backend logging fails
  //     console.error('Failed to send log to backend:', error);
  //   }
  // }
}

// Export singleton instance
export const logger = new FrontendLogger();

// Export convenience functions for easier migration
export const logDebug = (category: LogCategory, message: string, data?: any) =>
  logger.debug(category, message, data);

export const logInfo = (category: LogCategory, message: string, data?: any) =>
  logger.info(category, message, data);

export const logWarn = (category: LogCategory, message: string, data?: any) =>
  logger.warn(category, message, data);

export const logError = (category: LogCategory, message: string, data?: any) =>
  logger.error(category, message, data);
