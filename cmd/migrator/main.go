package main

import (
	"context"
	"flag"
	"log"
	"math/big"

	"github.com/citizenwallet/indexer/internal/config"
	"github.com/citizenwallet/indexer/internal/services/db"
)

func main() {
	log.Default().Println("postgres to sqlite migration migration...")

	chainId := flag.Int("chain", 1, "chain id")

	txBatch := flag.Int("txbatch", 1000, "tx batch size")

	token := flag.String("token", "", "token address")

	paymasterAddr := flag.String("paymaster", "", "paymaster address")

	env := flag.String("env", ".db.env", "path to .db.env file")

	dbpath := flag.String("dbpath", ".", "path to db")

	flag.Parse()

	if chainId == nil {
		log.Fatal("chainId is required")
	}

	if token == nil || *token == "" {
		log.Fatal("token is required")
	}

	if paymasterAddr == nil || *paymasterAddr == "" {
		log.Fatal("paymaster address is required")
	}

	chid := big.NewInt(int64(*chainId))

	ctx := context.Background()

	conf, err := config.NewDBConfig(ctx, *env)
	if err != nil {
		log.Fatal(err)
	}

	pqdb, err := db.NewPostgresDB(chid, conf.DBUser, conf.DBPassword, conf.DBName, conf.DBHost, conf.DBReaderHost, conf.DBSecret)
	if err != nil {
		log.Fatal(err)
	}
	defer pqdb.Close()

	db, err := db.NewDB(chid, *dbpath, conf.DBSecret)
	if err != nil {
		log.Fatal(err)
	}

	err = pqdb.Migrate(db, *token, *paymasterAddr, *txBatch)
	if err != nil {
		log.Fatal(err)
	}

	log.Default().Println("migration completed")
}
