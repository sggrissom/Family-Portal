package family

import (
	"context"
	"errors"
	"family/backend"
	"family/cfg"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

var Info vbolt.Info

const (
	// Keep request headers bounded and short-lived without imposing a global
	// timeout on photo uploads or long-running WebSocket connections.
	//
	// ReadTimeout and WriteTimeout are deliberately absent. Both would apply to
	// hijacked WebSocket connections, which coder/websocket does not clear the
	// deadlines on, and neither could be sized for a 1 KiB login and a 512 MiB
	// import at once. Per-route deadlines live in backend/request_timeouts.go.
	serverReadHeaderTimeout = 10 * time.Second
	serverIdleTimeout       = 2 * time.Minute
	serverMaxHeaderBytes    = 1 << 20 // 1 MiB
	serverShutdownTimeout   = 30 * time.Second
)

// NewHTTPServer applies the shared HTTP transport limits used by local and
// release servers. Handler-specific limits remain responsible for request
// bodies because uploads and JSON calls have different requirements.
func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		IdleTimeout:       serverIdleTimeout,
		MaxHeaderBytes:    serverMaxHeaderBytes,
	}
}

// RunHTTPServer serves requests until the context is canceled, then takes the
// process down in the order the pieces depend on each other:
//
//  1. Chat connections are closed with a Going Away frame. This has to come
//     first: an upgraded WebSocket has hijacked its connection, and
//     http.Server.Shutdown neither tracks nor waits for those, so anything left
//     open here is severed without warning when the process exits.
//  2. The HTTP server drains. No new requests are accepted; in-flight ones
//     finish. Once this returns, nothing can queue new background work.
//  3. The background workers stop, finishing what they had already accepted.
//
// Callers should derive ctx with signal.NotifyContext for SIGINT and SIGTERM.
// An error means the server itself failed; a worker that ran out of drain
// budget is logged rather than returned, because the process still exited
// having done everything it could.
func RunHTTPServer(ctx context.Context, server *http.Server) error {
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		// The listener died on its own — a port taken, a file descriptor
		// exhausted. Nothing was signalled, so the workers still need stopping
		// before the caller exits nonzero.
		backend.ShutdownWorkers(context.Background())
		return err
	case <-ctx.Done():
	}

	backend.ShutdownChatConnections(context.Background())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownCtx)

	backend.ShutdownWorkers(context.Background())

	if shutdownErr != nil {
		return shutdownErr
	}

	err := <-serveErr
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func OpenDB(dbpath string) *vbolt.DB {
	dbConnection := vbolt.Open(dbpath)
	vbolt.InitBuckets(dbConnection, &cfg.Info)

	// Migration: Populate search index for existing milestones
	vbolt.ApplyDBProcess(dbConnection, "2025-1004-populate-milestone-search", func() {
		vbolt.WithWriteTx(dbConnection, func(tx *vbolt.Tx) {
			// Iterate all existing milestones
			vbolt.IterateAll(tx, backend.MilestoneBkt, func(key int, milestone backend.Milestone) bool {
				// Populate search index for each milestone
				backend.UpdateMilestoneSearchIndex(tx, milestone)
				return true // Continue iteration
			})
			vbolt.TxCommit(tx)
		})
	})

	// Migration: one FamilyMembership row per user, mirroring User.FamilyId.
	// Additive only — nothing reads memberships until Stage 3 of the
	// multi-family plan (docs/multi-family-plan.md).
	vbolt.ApplyDBProcess(dbConnection, "2026-0804-backfill-family-membership", func() {
		vbolt.WithWriteTx(dbConnection, func(tx *vbolt.Tx) {
			backend.BackfillFamilyMemberships(tx)
			vbolt.TxCommit(tx)
		})
	})

	// Migration: one PersonFamily roster row per person, mirroring
	// Person.FamilyId and Person.Type. Stage 4 of the multi-family plan
	// (docs/multi-family-plan.md) — this table is what GetFamilyPeople reads,
	// so it must be populated before rosters resolve.
	vbolt.ApplyDBProcess(dbConnection, "2026-0804-backfill-person-family", func() {
		vbolt.WithWriteTx(dbConnection, func(tx *vbolt.Tx) {
			backend.BackfillPersonFamilies(tx)
			vbolt.TxCommit(tx)
		})
	})

	return dbConnection
}

// readinessHandler verifies that the application's durable dependencies are
// usable. Unlike /healthz, this endpoint can be removed from a load balancer
// while the process remains alive and able to recover.
func readinessHandler(db *vbolt.DB, staticDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		if db == nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		tx, err := db.Begin(false)
		if err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		_ = tx.Rollback()

		probe, err := os.CreateTemp(filepath.Clean(staticDir), ".ready-*")
		if err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		probePath := probe.Name()
		if err := probe.Close(); err != nil {
			_ = os.Remove(probePath)
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		if err := os.Remove(probePath); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

func MakeApplication() *vbeam.Application {
	// Load environment variables from .env file
	var err error
	if cfg.IsRelease {
		err = godotenv.Load("/srv/apps/family/shared/.env")
	} else {
		err = godotenv.Load()
	}
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Initialize rotating file logger only in production
	if cfg.IsRelease {
		vbeam.InitRotatingLogger(backend.LogFileBaseName)
	}

	backend.EnforceProductionConfig(cfg.DBPath, cfg.StaticDir)

	// Log application startup. The version is read from cfg rather than written
	// here, so the log line and the diagnostics view cannot disagree about what
	// is running; commit and build time are stamped in by the linker.
	build := cfg.Build()
	backend.LogInfo(backend.LogCategorySystem, "Family Record application starting", map[string]interface{}{
		"version":   build.Version,
		"commit":    build.Commit,
		"buildTime": build.BuildTime,
		"release":   build.Release,
		"dbPath":    cfg.DBPath,
		"staticDir": cfg.StaticDir,
	})

	db := OpenDB(cfg.DBPath)
	var app = vbeam.NewApplication("Family Record", db)

	backend.SetupAuth(app)
	backend.RegisterUserMethods(app)
	backend.RegisterAccountHandlers(app)
	backend.RegisterMembershipMethods(app)
	backend.RegisterPasswordResetMethods(app)
	backend.RegisterFamilyLinkMethods(app)
	backend.RegisterPersonMethods(app)
	backend.RegisterGrowthMethods(app)
	backend.RegisterMilestoneMethods(app)
	backend.RegisterActivityMethods(app)
	backend.RegisterActivityResultMethods(app)
	backend.RegisterActivityViewMethods(app)
	backend.RegisterActivityPhotoMethods(app)
	backend.RegisterTagMethods(app)
	backend.RegisterChatMethods(app)
	backend.RegisterPhotoMethods(app)
	backend.RegisterImportMethods(app)
	backend.RegisterExportMethods(app)
	backend.RegisterAIImportMethods(app)
	backend.RegisterAdminMethods(app)
	backend.RegisterDiagnosticsMethods(app)
	backend.RegisterSEOHandlers(app)
	backend.RegisterUniversalLinkHandlers(app)
	backend.RegisterPushNotificationMethods(app)
	backend.RegisterNotificationPreferenceMethods(app)
	backend.RegisterMobileVersionMethods(app)

	app.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	app.HandleFunc("GET /readyz", readinessHandler(app.DB, cfg.StaticDir))
	backend.RegisterBackupHandlers(app)

	// Initialize background photo processing worker
	backend.InitializePhotoWorker(100, app.DB) // Queue size of 100 jobs

	// Initialize background face analysis worker
	backend.InitializeAnalysisWorker(app.DB)

	// Initialize background push notification worker
	backend.InitializePushWorker(100, app.DB) // Queue size of 100 jobs

	// Initialize background outbound mail worker
	backend.InitializeMailWorker(100) // Queue size of 100 messages

	return app
}

// WrapApplication applies the standard middleware chain. Every server that
// serves this application to a network builds its handler here, so a wrapper
// added later cannot end up on one entry point and not another.
//
// Order matters. The correlation id is outermost, so even a request the rate
// limiter refuses can be found in the log by the code its response carried.
// Rate limiting comes next, so a flood is refused before any body is read or
// any handler touches the database. Request deadlines follow — a refused
// request never needed one — and they must be outside the security wrapper,
// which dispatches WebSocket upgrades itself and would otherwise leave those
// connections carrying whatever deadline the last request set. The bearer
// token wrapper is innermost, next to the dispatch it exists to feed: it
// rewrites a header and nothing else, so nothing outside it needs to know.
func WrapApplication(app *vbeam.Application) http.Handler {
	return backend.NewRequestIDWrapper(
		backend.NewRateLimitWrapper(
			backend.NewRequestTimeoutWrapper(
				backend.NewRequestSizeLimitWrapper(
					backend.NewBearerTokenWrapper(
						backend.NewSecurityWrapper(app))))))
}

func MakeSecureApplication() http.Handler {
	return WrapApplication(MakeApplication())
}
