package family

import (
	"family/cfg"

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
	db := OpenDB(cfg.DBPath)
	var app = vbeam.NewApplication("FamilyPortal", db)
	return app
}

