package main

import (
	"log"

	"ventus-inc/Ventus_Office_ReserveBot/api"
	"ventus-inc/Ventus_Office_ReserveBot/util"

	"github.com/dgraph-io/badger/v3"
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
	server.Start("5000")
}
