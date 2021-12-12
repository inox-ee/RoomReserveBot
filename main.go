package main

import (
	"log"

	"github.com/dgraph-io/badger/v3"
	"github.com/inox-ee/TestSlack/api"
	"github.com/inox-ee/TestSlack/util"
)

func main() {
	config, err := util.LoadConfig(".", "app")
	if err != nil {
		log.Fatal("cannot load config: ", err)
	}

	db, err := badger.Open(badger.DefaultOptions("/tmp/badger"))
	if err != nil {
		log.Fatal("cannot open badgerDB: ", err)
	}
	defer db.Close()

	server := api.NewServer(config, db)
	server.Start("8080")
}
