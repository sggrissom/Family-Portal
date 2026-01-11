package family

import (
	"family/backend"
	"family/cfg"
	"log"
	"net/http"

	"github.com/joho/godotenv"
	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

var Info vbolt.Info

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
	backend.RegisterChatMethods(app)
	backend.RegisterPhotoMethods(app)
	backend.RegisterImportMethods(app)
	backend.RegisterExportMethods(app)
	backend.RegisterAIImportMethods(app)
	backend.RegisterAdminMethods(app)
	backend.RegisterSEOHandlers(app)

	app.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Initialize background photo processing worker
	backend.InitializePhotoWorker(100, app.DB) // Queue size of 100 jobs

	return app
}

func MakeSecureApplication() http.Handler {
	app := MakeApplication()
	return backend.NewSecurityWrapper(app)
}
