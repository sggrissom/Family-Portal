import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as vlens from "vlens";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch, requireAuthInView } from "../../lib/authHelpers";
import "./logs-styles";

// Local type definitions (until server types are generated)
interface LogFileInfo {
  name: string;
  size: number;
  modTime: string;
  isToday: boolean;
  sizeString: string;
}


const entriesPerPage = 100;

interface LogsPageData {
  files: server.LogFileInfo[];
  stats: server.LogStats | null;
  initialEntries: server.PublicLogEntry[];
  initialFile: string;
  error: string;
}

export async function fetch(route: string, prefix: string): Promise<rpc.Response<LogsPageData>> {
  if (!await ensureAuthInFetch()) {
    return rpc.ok({
      files: [],
      stats: null,
      initialEntries: [],
      initialFile: "",
      error: ""
    });
  }

  try {
    // Fetch log files and stats
    const [filesResults, statsResults] = await Promise.all([
      server.GetLogFiles({}),
      server.GetLogStats({})
    ]);

    const [filesResult, fileErr] = filesResults;
    const [statsResult, statsErr] = statsResults;

    if (fileErr) {
      console.warn("Failed to load log files:", fileErr);
    }
    if (statsErr) {
      console.warn("Failed to load log stats:", statsErr);
    }

    const files = (filesResult && !fileErr) ? (filesResult.files || []) : [];
    const stats = (statsResult && !statsErr) ? (statsResult.stats || null) : null;
    let initialEntries: server.PublicLogEntry[] = [];
    let initialFile = "";


    // If we have files, select the most recent and load its content
    if (files.length > 0) {
      initialFile = files[0].name;

      try {
        const [contentResult, contentErr] = await server.GetLogContent({
          filename: initialFile,
          level: "",
          category: "",
          limit: entriesPerPage,
          offset: 0
        });
        if (!contentErr && contentResult) {
          initialEntries = contentResult.entries || [];
        } else {
          console.warn("Failed to load initial log content:", contentErr);
        }
      } catch (contentErr) {
        // If content loading fails, still show the file list
        console.warn("Failed to load initial log content:", contentErr);
      }
    }

    return rpc.ok({
      files,
      stats,
      initialEntries,
      initialFile,
      error: ""
    });
  } catch (err) {
    return rpc.ok({
      files: [],
      stats: null,
      initialEntries: [],
      initialFile: "",
      error: String(err)
    });
  }
}

// Component state type for logs page
interface LogsPageState {
  currentFile: string;
  logEntries: server.PublicLogEntry[];
  loading: boolean;
  error: string;
  levelFilter: string;
  categoryFilter: string;
  currentPage: number;
  totalLines: number;
  hasMore: boolean;
}

// Component state hook for logs page
const logsPageHook = vlens.declareHook((): LogsPageState => ({
  currentFile: "",
  logEntries: [],
  loading: false,
  error: "",
  levelFilter: "",
  categoryFilter: "",
  currentPage: 1,
  totalLines: 0,
  hasMore: false
}));

async function loadLogContent(filename: string, levelFilter: string, categoryFilter: string, currentPage: number) {
  const state = logsPageHook();
  if (!filename) return;

  state.loading = true;
  vlens.scheduleRedraw();

  try {
    const [result, err] = await server.GetLogContent({
      filename: filename,
      level: levelFilter || "",
      category: categoryFilter || "",
      limit: entriesPerPage,
      offset: (currentPage - 1) * entriesPerPage
    });

    if (!err && result) {
      state.logEntries = result.entries || [];
      state.totalLines = result.totalLines || 0;
      state.hasMore = result.hasMore || false;
      state.error = "";
    } else {
      state.logEntries = []; // Ensure it's always an array
      state.error = err || "Failed to load log content";
    }
  } catch (err) {
    state.logEntries = []; // Ensure it's always an array
    state.error = String(err);
  } finally {
    state.loading = false;
    vlens.scheduleRedraw();
  }
}

function handleFileChange(filename: string) {
  const state = logsPageHook();
  state.currentFile = filename;
  state.currentPage = 1;
  state.logEntries = [];
  loadLogContent(filename, state.levelFilter, state.categoryFilter, 1);
  vlens.scheduleRedraw();
}

function handleFilterChange() {
  const state = logsPageHook();
  state.currentPage = 1;
  loadLogContent(state.currentFile, state.levelFilter, state.categoryFilter, 1);
  vlens.scheduleRedraw();
}

function handlePageChange(newPage: number) {
  const state = logsPageHook();
  state.currentPage = newPage;
  loadLogContent(state.currentFile, state.levelFilter, state.categoryFilter, newPage);
  vlens.scheduleRedraw();
}

function formatTimestamp(timestamp: string): string {
  try {
    const date = new Date(timestamp);
    return date.toLocaleString();
  } catch {
    return timestamp;
  }
}

function getLevelBadgeClass(level: string): string {
  switch (level.toUpperCase()) {
    case "ERROR": return "log-badge log-badge-error";
    case "WARN": return "log-badge log-badge-warn";
    case "INFO": return "log-badge log-badge-info";
    case "DEBUG": return "log-badge log-badge-debug";
    default: return "log-badge log-badge-default";
  }
}

function getCategoryBadgeClass(category: string): string {
  return `log-category log-category-${category.toLowerCase()}`;
}

export function view(route: string, prefix: string, data: LogsPageData): preact.ComponentChild {
  const userAuth = requireAuthInView();
  if (!userAuth || !userAuth.isAdmin) {
    return preact.h("div", {}, [
      preact.h(Header, { isHome: false }),
      preact.h("main", { id: "app", className: "page-container" }, [
        preact.h("div", { className: "error-page" }, [
          preact.h("h1", {}, "Access Denied"),
          preact.h("p", {}, "Admin access required")
        ])
      ]),
      preact.h(Footer, {})
    ]);
  }

  // Initialize state from fetched data
  const state = logsPageHook();

  // Initialize state on first render if needed
  if (state.currentFile === "" && data.initialFile) {
    state.currentFile = data.initialFile;
    state.logEntries = data.initialEntries;
  }

  const fileSelectOptions = [
    preact.h("option", { value: "" }, "Select a log file..."),
    ...data.files.map(file =>
      preact.h("option", {
        key: file.name,
        value: file.name
      }, `${file.name} (${file.sizeString}) ${file.isToday ? "📍" : ""}`)
    )
  ];

  const logRows = state.logEntries.map((entry, index) =>
    preact.h("div", {
      key: index,
      className: "table-row"
    }, [
      preact.h("div", { className: "col-timestamp" }, formatTimestamp(entry.timestamp)),
      preact.h("div", { className: "col-level" },
        preact.h("span", { className: getLevelBadgeClass(entry.level) }, entry.level)
      ),
      preact.h("div", { className: "col-category" },
        preact.h("span", { className: getCategoryBadgeClass(entry.category) }, entry.category)
      ),
      preact.h("div", { className: "col-message" }, [
        entry.message,
        entry.data ? preact.h("div", { className: "log-data" },
          preact.h("pre", {}, JSON.stringify(entry.data, null, 2))
        ) : null
      ]),
      preact.h("div", { className: "col-user" },
        entry.userId ? `User ${entry.userId}` : "-"
      )
    ])
  );

  return preact.h("div", {}, [
    preact.h(Header, { isHome: false }),
    preact.h("main", { id: "app", className: "page-container" }, [
      preact.h("div", { className: "logs-page" }, [
        // Header
        preact.h("div", { className: "logs-header" }, [
          preact.h("h1", {}, "System Logs"),
          preact.h("div", { className: "logs-nav" },
            preact.h("a", { href: "/admin", className: "back-link" }, "← Back to Admin")
          )
        ]),

        // Error message
        (state.error || data.error) ? preact.h("div", { className: "error-message" }, [
          preact.h("span", { className: "error-icon" }, "⚠️"),
          preact.h("span", {}, state.error || data.error)
        ]) : null,

        // Statistics
        data.stats ? preact.h("div", { className: "logs-stats" },
          preact.h("div", { className: "stats-grid" }, [
            preact.h("div", { className: "stat-card" }, [
              preact.h("div", { className: "stat-number" }, data.stats.totalFiles),
              preact.h("div", { className: "stat-label" }, "Log Files")
            ]),
            preact.h("div", { className: "stat-card" }, [
              preact.h("div", { className: "stat-number" }, Math.round(data.stats.totalSize / 1024) + "KB"),
              preact.h("div", { className: "stat-label" }, "Total Size")
            ]),
            preact.h("div", { className: "stat-card" }, [
              preact.h("div", { className: "stat-number" }, data.stats.byLevel["ERROR"] || 0),
              preact.h("div", { className: "stat-label" }, "Errors")
            ]),
            preact.h("div", { className: "stat-card" }, [
              preact.h("div", { className: "stat-number" }, (data.stats.recent || []).length),
              preact.h("div", { className: "stat-label" }, "Recent Entries")
            ])
          ])
        ) : null,

        // Controls
        preact.h("div", { className: "logs-controls" }, [
          preact.h("div", { className: "logs-filters" }, [
            preact.h("div", { className: "filter-group" }, [
              preact.h("label", { htmlFor: "file-select" }, "Log File:"),
              preact.h("select", {
                id: "file-select",
                value: state.currentFile,
                onChange: (e: any) => handleFileChange(e.currentTarget.value)
              }, fileSelectOptions)
            ]),
            preact.h("div", { className: "filter-group" }, [
              preact.h("label", { htmlFor: "level-filter" }, "Level:"),
              preact.h("select", {
                id: "level-filter",
                value: state.levelFilter,
                onChange: (e: any) => {
                  state.levelFilter = e.currentTarget.value;
                  handleFilterChange();
                }
              }, [
                preact.h("option", { value: "" }, "All Levels"),
                preact.h("option", { value: "ERROR" }, "Errors"),
                preact.h("option", { value: "WARN" }, "Warnings"),
                preact.h("option", { value: "INFO" }, "Info"),
                preact.h("option", { value: "DEBUG" }, "Debug")
              ])
            ]),
            preact.h("div", { className: "filter-group" }, [
              preact.h("label", { htmlFor: "category-filter" }, "Category:"),
              preact.h("select", {
                id: "category-filter",
                value: state.categoryFilter,
                onChange: (e: any) => {
                  state.categoryFilter = e.currentTarget.value;
                  handleFilterChange();
                }
              }, [
                preact.h("option", { value: "" }, "All Categories"),
                preact.h("option", { value: "AUTH" }, "Authentication"),
                preact.h("option", { value: "PHOTO" }, "Photos"),
                preact.h("option", { value: "ADMIN" }, "Admin"),
                preact.h("option", { value: "API" }, "API"),
                preact.h("option", { value: "WORKER" }, "Background Jobs"),
                preact.h("option", { value: "SYSTEM" }, "System")
              ])
            ])
          ]),

          // Pagination
          state.currentFile ? preact.h("div", { className: "logs-pagination" }, [
            preact.h("span", { className: "pagination-info" },
              `Page ${state.currentPage} of ${Math.ceil(state.totalLines / entriesPerPage)} (${state.totalLines} total entries)`
            ),
            preact.h("div", { className: "pagination-controls" }, [
              preact.h("button", {
                onClick: () => handlePageChange(state.currentPage - 1),
                disabled: state.currentPage <= 1,
                className: "pagination-btn"
              }, "Previous"),
              preact.h("button", {
                onClick: () => handlePageChange(state.currentPage + 1),
                disabled: !state.hasMore,
                className: "pagination-btn"
              }, "Next")
            ])
          ]) : null
        ]),

        // Loading message
        state.loading ? preact.h("div", { className: "loading-message" }, [
          preact.h("span", { className: "loading-spinner" }, "⏳"),
          preact.h("span", {}, "Loading logs...")
        ]) : null,

        // Log content
        state.logEntries.length > 0 ? preact.h("div", { className: "logs-content" },
          preact.h("div", { className: "logs-table" }, [
            preact.h("div", { className: "table-header" }, [
              preact.h("div", { className: "col-timestamp" }, "Timestamp"),
              preact.h("div", { className: "col-level" }, "Level"),
              preact.h("div", { className: "col-category" }, "Category"),
              preact.h("div", { className: "col-message" }, "Message"),
              preact.h("div", { className: "col-user" }, "User")
            ]),
            ...logRows
          ])
        ) : null,

        // Empty state
        state.currentFile && state.logEntries.length === 0 && !state.loading ? preact.h("div", { className: "empty-logs" },
          preact.h("p", {}, "No log entries found matching the current filters.")
        ) : null
      ])
    ]),
    preact.h(Footer, {})
  ]);
}
