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
	return dbConnection
}

func MakeApplication() *vbeam.Application {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	db := OpenDB(cfg.DBPath)
	var app = vbeam.NewApplication("FamilyPortal", db)

	backend.SetupAuth(app)
	backend.RegisterUserMethods(app)
	backend.RegisterPersonMethods(app)
	backend.RegisterGrowthMethods(app)
	backend.RegisterMilestoneMethods(app)
	backend.RegisterPhotoMethods(app)
	backend.RegisterImportMethods(app)
	backend.RegisterAdminMethods(app)
	backend.RegisterSEOHandlers(app)

	return app
}

func MakeSecureApplication() http.Handler {
	app := MakeApplication()
	return backend.NewSecurityWrapper(app)
}

