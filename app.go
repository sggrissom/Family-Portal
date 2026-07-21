package family

import (
	"family/backend"
	"family/cfg"
	"log"
	"net/http"
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
