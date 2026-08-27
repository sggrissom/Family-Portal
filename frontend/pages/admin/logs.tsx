import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as vlens from "vlens";
import * as auth from "../../lib/authCache";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { ensureAuthInFetch } from "../../lib/authHelpers";
import { adminView } from "../../components/AdminGuard";
import { logWarn } from "../../lib/logger";
import "./logs-styles";

const entriesPerPage = 100;

interface LogsPageData {
  files: server.LogFileInfo[];
  stats: server.LogStats | null;
  initialEntries: server.PublicLogEntry[];
  initialFile: string;
  error: string;
}

export async function fetch(route: string, prefix: string): Promise<rpc.Response<LogsPageData>> {
  if (!(await ensureAuthInFetch())) {
    return rpc.ok({
      files: [],
      stats: null,
      initialEntries: [],
      initialFile: "",
      error: "",
    });
  }

  try {
    const [filesResults, statsResults] = await Promise.all([
      server.GetLogFiles({}),
      server.GetLogStats({}),
    ]);

    const [filesResult, fileErr] = filesResults;
    const [statsResult, statsErr] = statsResults;

    if (fileErr) {
      logWarn("admin", "Failed to load log files", fileErr);
    }
    if (statsErr) {
      logWarn("admin", "Failed to load log stats", statsErr);
    }

    const files = filesResult && !fileErr ? filesResult.files || [] : [];
    const stats = statsResult && !statsErr ? statsResult.stats || null : null;
    let initialEntries: server.PublicLogEntry[] = [];
    let initialFile = "";

    if (files.length > 0) {
      initialFile = files[0].name;

      try {
        const [contentResult, contentErr] = await server.GetLogContent({
          filename: initialFile,
          level: "",
          category: "",
          search: "",
          sinceHours: 0,
          limit: entriesPerPage,
          offset: 0,
          minDuration: null,
          sortBy: "time",
          sortDesc: true,
        });
        if (!contentErr && contentResult) {
          initialEntries = contentResult.entries || [];
        } else {
          logWarn("admin", "Failed to load initial log content", contentErr);
        }
      } catch (contentErr) {
        logWarn("admin", "Failed to load initial log content", contentErr);
      }
    }

    return rpc.ok({
      files,
      stats,
      initialEntries,
      initialFile,
      error: "",
    });
  } catch (err) {
    return rpc.ok({
      files: [],
      stats: null,
      initialEntries: [],
      initialFile: "",
      error: String(err),
    });
  }
}

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
  minDurationFilter: string;
  sortBy: string;
  sortDesc: boolean;
  searchText: string;
  sinceHours: number;
  filesSearched: string[];
  referenceInput: string;
  referenceResult: server.LookupLogReferenceResponse | null;
  referenceLoading: boolean;
  referenceError: string;
}

const logsPageHook = vlens.declareHook(
  (): LogsPageState => ({
    currentFile: "",
    logEntries: [],
    loading: false,
    error: "",
    levelFilter: "",
    categoryFilter: "",
    currentPage: 1,
    totalLines: 0,
    hasMore: false,
    minDurationFilter: "",
    sortBy: "time",
    sortDesc: true,
    searchText: "",
    sinceHours: 0,
    filesSearched: [],
    referenceInput: "",
    referenceResult: null,
    referenceLoading: false,
    referenceError: "",
  })
);

async function loadLogContent() {
  const state = logsPageHook();

  state.loading = true;
  vlens.scheduleRedraw();

  try {
    const minDurationMicros = state.minDurationFilter.trim()
      ? parseInt(state.minDurationFilter) * 1000
      : undefined;

    const [result, err] = await server.GetLogContent({
      filename: state.searchText.trim() ? "" : state.currentFile,
      level: state.levelFilter || "",
      category: state.categoryFilter || "",
      search: state.searchText.trim(),
      sinceHours: state.sinceHours,
      limit: entriesPerPage,
      offset: (state.currentPage - 1) * entriesPerPage,
      minDuration: minDurationMicros || null,
      sortBy: state.sortBy || "time",
      sortDesc: state.sortDesc,
    });

    if (!err && result) {
      state.logEntries = result.entries || [];
      state.totalLines = result.totalLines || 0;
      state.hasMore = result.hasMore || false;
      state.filesSearched = result.filesSearched || [];
      state.error = "";
    } else {
      state.logEntries = [];
      state.error = err || "Failed to load log content";
    }
  } catch (err) {
    state.logEntries = [];
    state.error = String(err);
  } finally {
    state.loading = false;
    vlens.scheduleRedraw();
  }
}

async function lookUpReference() {
  const state = logsPageHook();
  if (!state.referenceInput.trim()) {
    state.referenceResult = null;
    state.referenceError = "";
    vlens.scheduleRedraw();
    return;
  }

  state.referenceLoading = true;
  state.referenceError = "";
  vlens.scheduleRedraw();

  const [result, err] = await server.LookupLogReference({
    reference: state.referenceInput,
    context: 10,
  });

  state.referenceLoading = false;
  if (err) {
    state.referenceResult = null;
    state.referenceError = err;
  } else {
    state.referenceResult = result ?? null;
    state.referenceError = "";
  }
  vlens.scheduleRedraw();
}

function clearReference() {
  const state = logsPageHook();
  state.referenceInput = "";
  state.referenceResult = null;
  state.referenceError = "";
  vlens.scheduleRedraw();
}

function showRecentErrors() {
  const state = logsPageHook();
  state.levelFilter = "ERROR";
  state.categoryFilter = "";
  state.minDurationFilter = "";
  state.searchText = "";
  state.sinceHours = 24;
  state.sortBy = "time";
  state.sortDesc = true;
  state.currentFile = "";
  handleFilterChange();
}

function handleFileChange(filename: string) {
  const state = logsPageHook();
  state.currentFile = filename;
  state.logEntries = [];
  handleFilterChange();
}

function handleFilterChange() {
  const state = logsPageHook();
  state.currentPage = 1;
  state.totalLines = 0;
  state.hasMore = false;
  loadLogContent();
  vlens.scheduleRedraw();
}

function handlePageChange(newPage: number) {
  const state = logsPageHook();

  const totalPages = Math.ceil(state.totalLines / entriesPerPage);
  if (newPage < 1 || (totalPages > 0 && newPage > totalPages)) {
    return;
  }

  state.currentPage = newPage;
  loadLogContent();
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

function formatDuration(durationMicros: number | null | undefined): string {
  if (!durationMicros) return "-";

  if (durationMicros < 1000) {
    return `${durationMicros}µs`;
  } else if (durationMicros < 1000000) {
    return `${(durationMicros / 1000).toFixed(2)}ms`;
  } else {
    return `${(durationMicros / 1000000).toFixed(2)}s`;
  }
}

function getDurationColor(durationMicros: number | null | undefined): string {
  if (!durationMicros) return "";

  if (durationMicros >= 1000000) return "duration-slow";
  if (durationMicros >= 100000) return "duration-medium";
  return "duration-fast";
}

function getLevelBadgeClass(level: string): string {
  switch (level.toUpperCase()) {
    case "ERROR":
      return "log-badge log-badge-error";
    case "WARN":
      return "log-badge log-badge-warn";
    case "INFO":
      return "log-badge log-badge-info";
    case "DEBUG":
      return "log-badge log-badge-debug";
    default:
      return "log-badge log-badge-default";
  }
}

function getCategoryBadgeClass(category: string): string {
  return `log-category log-category-${category.toLowerCase()}`;
}

function PerformanceStatsPanel({ stats }: { stats: server.PerformanceStats | undefined }) {
  if (!stats || stats.totalRequests === 0) {
    return preact.h("div", { className: "performance-panel" }, [
      preact.h("h3", {}, "Performance Statistics"),
      preact.h("p", { className: "no-data" }, "No performance data available"),
    ]);
  }

  return preact.h("div", { className: "performance-panel" }, [
    preact.h("h3", {}, "Performance Statistics"),

    preact.h("div", { className: "perf-overview" }, [
      preact.h("div", { className: "perf-stat" }, [
        preact.h("span", { className: "perf-label" }, "Total Requests:"),
        preact.h("span", { className: "perf-value" }, stats.totalRequests.toLocaleString()),
      ]),
      preact.h("div", { className: "perf-stat" }, [
        preact.h("span", { className: "perf-label" }, "Average Response:"),
        preact.h("span", { className: "perf-value" }, formatDuration(stats.averageResponse)),
      ]),
      preact.h("div", { className: "perf-stat" }, [
        preact.h("span", { className: "perf-label" }, "Median Response:"),
        preact.h("span", { className: "perf-value" }, formatDuration(stats.medianResponse)),
      ]),
    ]),

    preact.h("div", { className: "perf-percentiles" }, [
      preact.h("h4", {}, "Response Time Percentiles"),
      preact.h("div", { className: "percentile-grid" }, [
        preact.h("div", { className: "percentile-item" }, [
          preact.h("span", { className: "percentile-label" }, "P90:"),
          preact.h("span", { className: "percentile-value" }, formatDuration(stats.p90Response)),
        ]),
        preact.h("div", { className: "percentile-item" }, [
          preact.h("span", { className: "percentile-label" }, "P95:"),
          preact.h("span", { className: "percentile-value" }, formatDuration(stats.p95Response)),
        ]),
        preact.h("div", { className: "percentile-item" }, [
          preact.h("span", { className: "percentile-label" }, "P99:"),
          preact.h("span", { className: "percentile-value" }, formatDuration(stats.p99Response)),
        ]),
      ]),
    ]),

    stats.slowestEndpoints.length > 0
      ? preact.h("div", { className: "slowest-endpoints" }, [
          preact.h("h4", {}, "Slowest Endpoints"),
          preact.h("div", { className: "endpoints-table" }, [
            preact.h("div", { className: "endpoints-header" }, [
              preact.h("div", { className: "col-endpoint" }, "Endpoint"),
              preact.h("div", { className: "col-requests" }, "Requests"),
              preact.h("div", { className: "col-avg" }, "Avg Response"),
              preact.h("div", { className: "col-max" }, "Max Response"),
              preact.h("div", { className: "col-error" }, "Error Rate"),
            ]),
            ...stats.slowestEndpoints
              .slice(0, 5)
              .map((endpoint, index) =>
                preact.h("div", { key: index, className: "endpoints-row" }, [
                  preact.h("div", { className: "col-endpoint" }, [
                    preact.h(
                      "span",
                      { className: `method-badge method-${endpoint.method.toLowerCase()}` },
                      endpoint.method
                    ),
                    preact.h("span", { className: "endpoint-path" }, endpoint.path),
                  ]),
                  preact.h("div", { className: "col-requests" }, endpoint.count.toLocaleString()),
                  preact.h(
                    "div",
                    { className: "col-avg" },
                    formatDuration(endpoint.averageResponse)
                  ),
                  preact.h("div", { className: "col-max" }, formatDuration(endpoint.maxResponse)),
                  preact.h("div", { className: "col-error" }, `${endpoint.errorRate.toFixed(1)}%`),
                ])
              ),
          ]),
        ])
      : null,
  ]);
}

function logEntryRow(entry: server.PublicLogEntry, key: string | number, extraClass?: string) {
  return preact.h("div", { key, className: extraClass ? `table-row ${extraClass}` : "table-row" }, [
    preact.h("div", { className: "col-timestamp" }, formatTimestamp(entry.timestamp)),
    preact.h(
      "div",
      { className: "col-level" },
      preact.h("span", { className: getLevelBadgeClass(entry.level) }, entry.level)
    ),
    preact.h(
      "div",
      { className: "col-category" },
      preact.h("span", { className: getCategoryBadgeClass(entry.category) }, entry.category)
    ),
    preact.h("div", { className: "col-duration" }, [
      entry.duration
        ? preact.h(
            "span",
            { className: `duration-badge ${getDurationColor(entry.duration)}` },
            formatDuration(entry.duration)
          )
        : "-",
      entry.handlerDuration && entry.duration && entry.handlerDuration !== entry.duration
        ? preact.h(
            "div",
            { className: "handler-duration" },
            `(${formatDuration(entry.handlerDuration)})`
          )
        : null,
    ]),
    preact.h("div", { className: "col-method" }, [
      entry.httpMethod
        ? preact.h(
            "span",
            { className: `method-badge method-${entry.httpMethod.toLowerCase()}` },
            entry.httpMethod
          )
        : "-",
      entry.httpPath ? preact.h("div", { className: "http-path" }, entry.httpPath) : null,
      entry.httpStatus
        ? preact.h(
            "span",
            { className: `status-badge status-${Math.floor(entry.httpStatus / 100)}xx` },
            entry.httpStatus
          )
        : null,
    ]),
    preact.h("div", { className: "col-message" }, [
      entry.message,
      entry.data
        ? preact.h(
            "div",
            { className: "log-data" },
            preact.h("pre", {}, JSON.stringify(entry.data, null, 2))
          )
        : null,
      entry.stackTrace
        ? preact.h("div", { className: "log-stack-trace" }, preact.h("pre", {}, entry.stackTrace))
        : null,
    ]),
    preact.h("div", { className: "col-user" }, entry.userId ? `User ${entry.userId}` : "-"),
  ]);
}

const logTableHeader = () =>
  preact.h("div", { className: "table-header" }, [
    preact.h("div", { className: "col-timestamp" }, "Timestamp"),
    preact.h("div", { className: "col-level" }, "Level"),
    preact.h("div", { className: "col-category" }, "Category"),
    preact.h("div", { className: "col-duration" }, "Duration"),
    preact.h("div", { className: "col-method" }, "Method"),
    preact.h("div", { className: "col-message" }, "Message"),
    preact.h("div", { className: "col-user" }, "User"),
  ]);

function ReferenceLookupPanel({ state }: { state: LogsPageState }) {
  const result = state.referenceResult;

  return preact.h("div", { className: "reference-panel" }, [
    preact.h("h3", {}, "Look up a reference code"),
    preact.h(
      "p",
      { className: "reference-hint" },
      "Paste the code from the error someone reported — or the whole sentence — to find what it was logged against."
    ),
    preact.h("div", { className: "reference-controls" }, [
      preact.h("input", {
        type: "search",
        className: "reference-input",
        placeholder: "e.g. a1b2c3d4e5f6",
        value: state.referenceInput,
        onInput: (e: any) => {
          state.referenceInput = e.currentTarget.value;
        },
        onKeyDown: (e: any) => {
          if (e.key === "Enter") lookUpReference();
        },
      }),
      preact.h(
        "button",
        { className: "pagination-btn", onClick: lookUpReference, disabled: state.referenceLoading },
        state.referenceLoading ? "Looking…" : "Look up"
      ),
      result || state.referenceError
        ? preact.h("button", { className: "pagination-btn", onClick: clearReference }, "Clear")
        : null,
    ]),

    state.referenceError
      ? preact.h("div", { className: "logs-error-message" }, [
          preact.h("span", { className: "error-icon" }, "⚠️"),
          preact.h("span", {}, state.referenceError),
        ])
      : null,

    result && !result.found
      ? preact.h(
          "div",
          { className: "empty-logs" },
          preact.h(
            "p",
            {},
            result.filesSearched.length > 0
              ? `Not found in ${result.filesSearched.length} log file${result.filesSearched.length === 1 ? "" : "s"} (${result.filesSearched.join(", ")}).`
              : "There are no log files to search."
          )
        )
      : null,

    result && result.found
      ? preact.h("div", { className: "reference-result" }, [
          preact.h("div", { className: "reference-found-in" }, `Found in ${result.file}`),
          preact.h("div", { className: "logs-table" }, [
            logTableHeader(),
            ...result.before.map((entry, i) => logEntryRow(entry, `b${i}`, "context-row")),
            logEntryRow(result.entry, "match", "reference-match"),
            ...result.after.map((entry, i) => logEntryRow(entry, `a${i}`, "context-row")),
          ]),
        ])
      : null,
  ]);
}

export function view(route: string, prefix: string, data: LogsPageData): preact.ComponentChild {
  return adminView(() => {
    const state = logsPageHook();

    if (!state.referenceInput && !state.referenceResult && !state.referenceLoading) {
      const fromUrl = new URLSearchParams(window.location.search).get("ref");
      if (fromUrl) {
        state.referenceInput = fromUrl;
        setTimeout(lookUpReference, 0);
      }
    }

    if (state.currentFile === "" && data.initialFile) {
      state.currentFile = data.initialFile;
      state.logEntries = data.initialEntries;
      if (data.initialEntries.length > 0) {
        setTimeout(loadLogContent, 0);
      }
    }

    const fileSelectOptions = [
      preact.h("option", { value: "" }, "Select a log file..."),
      ...data.files.map(file =>
        preact.h(
          "option",
          {
            key: file.name,
            value: file.name,
          },
          `${file.name} (${file.sizeString}) ${file.isToday ? "📍" : ""}`
        )
      ),
    ];

    const logRows = state.logEntries.map((entry, index) => logEntryRow(entry, index));

    return preact.h("div", {}, [
      preact.h(Header, { isHome: false }),
      preact.h("main", { id: "app", className: "page-container" }, [
        preact.h("div", { className: "logs-page" }, [
          preact.h("div", { className: "logs-header" }, [
            preact.h("h1", {}, "System Logs"),
            preact.h(
              "div",
              { className: "logs-nav" },
              preact.h("a", { href: "/admin", className: "logs-back-link" }, "← Back to Admin")
            ),
          ]),

          state.error || data.error
            ? preact.h("div", { className: "logs-error-message" }, [
                preact.h("span", { className: "error-icon" }, "⚠️"),
                preact.h("span", {}, state.error || data.error),
              ])
            : null,

          preact.h(ReferenceLookupPanel, { state }),

          data.stats
            ? preact.h(
                "div",
                { className: "logs-stats" },
                preact.h("div", { className: "logs-stats-grid" }, [
                  preact.h("div", { className: "logs-stat-card" }, [
                    preact.h("div", { className: "logs-stat-number" }, data.stats.totalFiles),
                    preact.h("div", { className: "logs-stat-label" }, "Log Files"),
                  ]),
                  preact.h("div", { className: "logs-stat-card" }, [
                    preact.h(
                      "div",
                      { className: "logs-stat-number" },
                      Math.round(data.stats.totalSize / 1024) + "KB"
                    ),
                    preact.h("div", { className: "logs-stat-label" }, "Total Size"),
                  ]),
                  preact.h("div", { className: "logs-stat-card" }, [
                    preact.h(
                      "div",
                      { className: "logs-stat-number" },
                      data.stats.byLevel["ERROR"] || 0
                    ),
                    preact.h("div", { className: "logs-stat-label" }, "Errors"),
                  ]),
                  preact.h("div", { className: "logs-stat-card" }, [
                    preact.h(
                      "div",
                      { className: "logs-stat-number" },
                      (data.stats.recent || []).length
                    ),
                    preact.h("div", { className: "logs-stat-label" }, "Recent Entries"),
                  ]),
                ])
              )
            : null,

          data.stats
            ? preact.h(PerformanceStatsPanel, { stats: data.stats.performanceStats })
            : null,

          preact.h("div", { className: "logs-controls" }, [
            preact.h("div", { className: "logs-filters" }, [
              preact.h("div", { className: "logs-filter-group" }, [
                preact.h("label", { htmlFor: "file-select" }, "Log File:"),
                preact.h(
                  "select",
                  {
                    id: "file-select",
                    value: state.currentFile,
                    onChange: (e: any) => handleFileChange(e.currentTarget.value),
                  },
                  fileSelectOptions
                ),
              ]),
              preact.h("div", { className: "logs-filter-group logs-filter-group-search" }, [
                preact.h("label", { htmlFor: "log-search" }, "Search:"),
                preact.h("input", {
                  id: "log-search",
                  type: "search",
                  placeholder: "text, path, or reference code — searches every file",
                  value: state.searchText,
                  onInput: (e: any) => {
                    state.searchText = e.currentTarget.value;
                  },
                  onKeyDown: (e: any) => {
                    if (e.key === "Enter") handleFilterChange();
                  },
                  onChange: () => handleFilterChange(),
                }),
              ]),
              preact.h("div", { className: "logs-filter-group" }, [
                preact.h("label", { htmlFor: "since-filter" }, "Time range:"),
                preact.h(
                  "select",
                  {
                    id: "since-filter",
                    value: String(state.sinceHours),
                    onChange: (e: any) => {
                      state.sinceHours = parseInt(e.currentTarget.value, 10);
                      handleFilterChange();
                    },
                  },
                  [
                    preact.h("option", { value: "0" }, "Everything"),
                    preact.h("option", { value: "1" }, "Last hour"),
                    preact.h("option", { value: "24" }, "Last 24 hours"),
                    preact.h("option", { value: "168" }, "Last 7 days"),
                  ]
                ),
              ]),
              preact.h("div", { className: "logs-filter-group" }, [
                preact.h("label", { htmlFor: "level-filter" }, "Level:"),
                preact.h(
                  "select",
                  {
                    id: "level-filter",
                    value: state.levelFilter,
                    onChange: (e: any) => {
                      state.levelFilter = e.currentTarget.value;
                      handleFilterChange();
                    },
                  },
                  [
                    preact.h("option", { value: "" }, "All Levels"),
                    preact.h("option", { value: "ERROR" }, "Errors"),
                    preact.h("option", { value: "WARN" }, "Warnings"),
                    preact.h("option", { value: "INFO" }, "Info"),
                    preact.h("option", { value: "DEBUG" }, "Debug"),
                  ]
                ),
              ]),
              preact.h("div", { className: "logs-filter-group" }, [
                preact.h("label", { htmlFor: "category-filter" }, "Category:"),
                preact.h(
                  "select",
                  {
                    id: "category-filter",
                    value: state.categoryFilter,
                    onChange: (e: any) => {
                      state.categoryFilter = e.currentTarget.value;
                      handleFilterChange();
                    },
                  },
                  [
                    preact.h("option", { value: "" }, "All Categories"),
                    preact.h("option", { value: "AUTH" }, "Authentication"),
                    preact.h("option", { value: "PHOTO" }, "Photos"),
                    preact.h("option", { value: "ADMIN" }, "Admin"),
                    preact.h("option", { value: "API" }, "API"),
                    preact.h("option", { value: "WORKER" }, "Background Jobs"),
                    preact.h("option", { value: "SYSTEM" }, "System"),
                  ]
                ),
              ]),

              preact.h("div", { className: "logs-filter-group performance-filters" }, [
                preact.h("label", { htmlFor: "duration-filter" }, "Min Duration (ms):"),
                preact.h("input", {
                  id: "duration-filter",
                  type: "number",
                  placeholder: "e.g. 100",
                  value: state.minDurationFilter,
                  onChange: (e: any) => {
                    state.minDurationFilter = e.currentTarget.value;
                    handleFilterChange();
                  },
                }),
              ]),
              preact.h("div", { className: "logs-filter-group" }, [
                preact.h("label", { htmlFor: "sort-filter" }, "Sort By:"),
                preact.h(
                  "select",
                  {
                    id: "sort-filter",
                    value: state.sortBy,
                    onChange: (e: any) => {
                      state.sortBy = e.currentTarget.value;
                      handleFilterChange();
                    },
                  },
                  [
                    preact.h("option", { value: "time" }, "Time"),
                    preact.h("option", { value: "duration" }, "Duration"),
                  ]
                ),
              ]),
              preact.h("div", { className: "logs-filter-group" }, [
                preact.h("label", { htmlFor: "sort-order" }, "Order:"),
                preact.h(
                  "select",
                  {
                    id: "sort-order",
                    value: state.sortDesc ? "desc" : "asc",
                    onChange: (e: any) => {
                      state.sortDesc = e.currentTarget.value === "desc";
                      handleFilterChange();
                    },
                  },
                  [
                    preact.h("option", { value: "asc" }, "Ascending"),
                    preact.h("option", { value: "desc" }, "Descending"),
                  ]
                ),
              ]),
            ]),

            preact.h("div", { className: "logs-presets" }, [
              preact.h(
                "button",
                { className: "pagination-btn", onClick: showRecentErrors },
                "Errors in the last 24h"
              ),
              state.searchText.trim() && state.filesSearched.length > 0
                ? preact.h(
                    "span",
                    { className: "pagination-info" },
                    `Searched ${state.filesSearched.length} file${state.filesSearched.length === 1 ? "" : "s"}`
                  )
                : null,
            ]),

            state.currentFile || state.searchText.trim()
              ? (() => {
                  const totalPages =
                    state.totalLines > 0 ? Math.ceil(state.totalLines / entriesPerPage) : 0;
                  const showPagination = totalPages > 1 || state.totalLines > entriesPerPage;

                  return showPagination
                    ? preact.h("div", { className: "logs-pagination" }, [
                        preact.h(
                          "span",
                          { className: "pagination-info" },
                          totalPages > 0
                            ? `Page ${state.currentPage} of ${totalPages} (${state.totalLines} total entries)`
                            : `${state.totalLines} total entries`
                        ),
                        totalPages > 1
                          ? preact.h("div", { className: "pagination-controls" }, [
                              preact.h(
                                "button",
                                {
                                  onClick: () => handlePageChange(state.currentPage - 1),
                                  disabled: state.currentPage <= 1,
                                  className: "pagination-btn",
                                },
                                "Previous"
                              ),
                              preact.h(
                                "button",
                                {
                                  onClick: () => handlePageChange(state.currentPage + 1),
                                  disabled: state.currentPage >= totalPages,
                                  className: "pagination-btn",
                                },
                                "Next"
                              ),
                            ])
                          : null,
                      ])
                    : null;
                })()
              : null,
          ]),

          state.loading
            ? preact.h("div", { className: "loading-message" }, [
                preact.h("span", { className: "loading-spinner" }, "⏳"),
                preact.h("span", {}, "Loading logs..."),
              ])
            : null,

          state.logEntries.length > 0
            ? preact.h(
                "div",
                { className: "logs-content" },
                preact.h("div", { className: "logs-table" }, [logTableHeader(), ...logRows])
              )
            : null,

          state.currentFile && state.logEntries.length === 0 && !state.loading
            ? preact.h(
                "div",
                { className: "empty-logs" },
                preact.h("p", {}, "No log entries found matching the current filters.")
              )
            : null,
        ]),
      ]),
      preact.h(Footer, {}),
    ]);
  });
}
