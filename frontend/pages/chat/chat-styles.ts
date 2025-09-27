import { block } from "vlens/css";

// Chat Page Container
block(`
.chat-container {
  max-width: 1000px;
  margin: 0 auto;
  padding: 20px;
  height: calc(100vh - 160px);
  display: flex;
  flex-direction: column;
}
`);

block(`
.chat-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg);
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid var(--border);
}
`);

// Chat Header
block(`
.chat-header {
  background: var(--surface);
  border-bottom: 1px solid var(--border);
  padding: 20px 24px;
  text-align: center;
}
`);

block(`
.chat-header h1 {
  font-size: 1.8rem;
  margin: 0 0 8px;
  color: var(--text);
  font-weight: 700;
}
`);

block(`
.chat-header p {
  font-size: 1rem;
  color: var(--muted);
  margin: 0;
}
`);

// Chat Content Area
block(`
.chat-content {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
`);

// Messages Container
block(`
.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 20px 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  scroll-behavior: smooth;
}
`);

block(`
.chat-messages::-webkit-scrollbar {
  width: 6px;
}
`);

block(`
.chat-messages::-webkit-scrollbar-track {
  background: var(--bg);
}
`);

block(`
.chat-messages::-webkit-scrollbar-thumb {
  background: var(--border);
  border-radius: 3px;
}
`);

block(`
.chat-messages::-webkit-scrollbar-thumb:hover {
  background: var(--muted);
}
`);

// Message Styles
block(`
.message {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  max-width: 80%;
  animation: fadeIn 0.2s ease-out;
}
`);

block(`
@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
`);

block(`
.message-own {
  align-self: flex-end;
  flex-direction: row-reverse;
}
`);

block(`
.message-other {
  align-self: flex-start;
}
`);

// Message Avatar
block(`
.message-avatar {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: var(--surface);
  border: 2px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
`);

block(`
.avatar-icon {
  font-size: 1.5rem;
}
`);

// Message Content
block(`
.message-content {
  flex: 1;
  min-width: 0;
}
`);

block(`
.message-sender {
  font-size: 0.8rem;
  color: var(--muted);
  margin-bottom: 4px;
  font-weight: 600;
}
`);

block(`
.message-own .message-sender {
  text-align: right;
}
`);

// Message Bubble
block(`
.message-bubble {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 18px;
  padding: 12px 16px;
  position: relative;
  max-width: 100%;
  word-wrap: break-word;
}
`);

block(`
.message-own .message-bubble {
  background: var(--primary-accent);
  border-color: var(--primary-accent);
  color: white;
}
`);

block(`
.message-text {
  margin: 0 0 6px;
  line-height: 1.4;
  font-size: 0.95rem;
}
`);

block(`
.message-own .message-text {
  color: white;
}
`);

block(`
.message-timestamp {
  font-size: 0.75rem;
  color: var(--muted);
  opacity: 0.8;
}
`);

block(`
.message-own .message-timestamp {
  color: rgba(255, 255, 255, 0.8);
}
`);

// Chat Input Form
block(`
.chat-input-form {
  background: var(--surface);
  border-top: 1px solid var(--border);
  padding: 16px 24px;
}
`);

block(`
.input-container {
  display: flex;
  gap: 12px;
  align-items: center;
}
`);

block(`
.message-input {
  flex: 1;
  padding: 12px 16px;
  border: 1px solid var(--border);
  border-radius: 24px;
  background: var(--bg);
  color: var(--text);
  font-size: 0.95rem;
  outline: none;
  transition: all 0.2s ease;
}
`);

block(`
.message-input:focus {
  border-color: var(--primary-accent);
  box-shadow: 0 0 0 2px rgba(105, 219, 124, 0.2);
}
`);

block(`
.message-input::placeholder {
  color: var(--muted);
}
`);

block(`
.send-button {
  background: var(--primary-accent);
  color: white;
  border: none;
  border-radius: 50%;
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  font-weight: 600;
  font-size: 0.9rem;
  transition: all 0.2s ease;
  flex-shrink: 0;
}
`);

block(`
.send-button:hover:not(:disabled) {
  background: var(--accent);
  transform: scale(1.05);
}
`);

block(`
.send-button:disabled {
  background: var(--border);
  cursor: not-allowed;
  transform: none;
}
`);

// Mobile Responsive Design
block(`
@media (max-width: 768px) {
  .chat-container {
    padding: 10px;
    height: calc(100vh - 140px);
  }

  .chat-header {
    padding: 16px 20px;
  }

  .chat-header h1 {
    font-size: 1.5rem;
  }

  .chat-messages {
    padding: 16px 20px;
    gap: 12px;
  }

  .message {
    max-width: 90%;
    gap: 10px;
  }

  .message-avatar {
    width: 36px;
    height: 36px;
  }

  .avatar-icon {
    font-size: 1.3rem;
  }

  .message-bubble {
    padding: 10px 14px;
    border-radius: 16px;
  }

  .message-text {
    font-size: 0.9rem;
  }

  .chat-input-form {
    padding: 12px 20px;
  }

  .input-container {
    gap: 10px;
  }

  .message-input {
    padding: 10px 14px;
    font-size: 0.9rem;
  }

  .send-button {
    width: 40px;
    height: 40px;
    font-size: 0.8rem;
  }
}
`);

block(`
@media (max-width: 480px) {
  .chat-container {
    padding: 5px;
    height: calc(100vh - 120px);
  }

  .chat-header {
    padding: 12px 16px;
  }

  .chat-header h1 {
    font-size: 1.3rem;
  }

  .chat-header p {
    font-size: 0.9rem;
  }

  .chat-messages {
    padding: 12px 16px;
    gap: 10px;
  }

  .message {
    max-width: 95%;
    gap: 8px;
  }

  .message-avatar {
    width: 32px;
    height: 32px;
  }

  .avatar-icon {
    font-size: 1.1rem;
  }

  .message-bubble {
    padding: 8px 12px;
    border-radius: 14px;
  }

  .message-text {
    font-size: 0.85rem;
  }

  .message-timestamp {
    font-size: 0.7rem;
  }

  .chat-input-form {
    padding: 10px 16px;
  }

  .send-button {
    width: 36px;
    height: 36px;
    font-size: 0.75rem;
  }
}
`);

// Dark theme specific adjustments
block(`
[data-theme="dark"] .message-own .message-bubble {
  background: var(--primary-accent);
  border-color: var(--primary-accent);
}
`);

block(`
[data-theme="dark"] .chat-messages::-webkit-scrollbar-thumb {
  background: var(--border);
}
`);

block(`
[data-theme="dark"] .chat-messages::-webkit-scrollbar-thumb:hover {
  background: var(--muted);
}
`);
