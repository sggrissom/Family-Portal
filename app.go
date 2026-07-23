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

// RunHTTPServer serves requests until the context is canceled. Cancellation
// stops new connections and gives in-flight HTTP requests time to finish.
// Callers should derive ctx with signal.NotifyContext for SIGINT and SIGTERM.
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
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
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
		vbeam.InitRotatingLogger("family_portal")
	}

	// Log application startup
	backend.LogInfo(backend.LogCategorySystem, "Family Portal application starting", map[string]interface{}{
		"version":   "1.0.0",
		"dbPath":    cfg.DBPath,
		"staticDir": cfg.StaticDir,
	})

	db := OpenDB(cfg.DBPath)
	var app = vbeam.NewApplication("FamilyPortal", db)

	backend.SetupAuth(app)
	backend.RegisterUserMethods(app)
	backend.RegisterPersonMethods(app)
	backend.RegisterGrowthMethods(app)
	backend.RegisterMilestoneMethods(app)
	backend.RegisterTagMethods(app)
	backend.RegisterChatMethods(app)
	backend.RegisterPhotoMethods(app)
	backend.RegisterImportMethods(app)
	backend.RegisterExportMethods(app)
	backend.RegisterAIImportMethods(app)
	backend.RegisterAdminMethods(app)
	backend.RegisterSEOHandlers(app)
	backend.RegisterPushNotificationMethods(app)
	backend.RegisterMobileVersionMethods(app)

	app.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	app.HandleFunc("GET /readyz", readinessHandler(app.DB, cfg.StaticDir))

	// Initialize background photo processing worker
	backend.InitializePhotoWorker(100, app.DB) // Queue size of 100 jobs

	// Initialize background face analysis worker
	backend.InitializeAnalysisWorker(app.DB)

	// Initialize background push notification worker
	backend.InitializePushWorker(100, app.DB) // Queue size of 100 jobs

	return app
}

func MakeSecureApplication() http.Handler {
	app := MakeApplication()
	return backend.NewSecurityWrapper(app)
}
