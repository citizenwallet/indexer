package govdb

import (
	"fmt"
	"github.com/citizenwallet/indexer/pkg/govindexer"
	"strings"
	"time"
)

type GovernorDB struct {
	p *DB
}

func (gdb *GovernorDB) Create() error {
	_, err := gdb.p.db.Exec(fmt.Sprintf(`
	CREATE TABLE %s(
		contract text NOT NULL,
		state text NOT NULL,
		created_at timestamp NOT NULL,
		updated_at timestamp NOT NULL,
		start_block integer NOT NULL,
		last_block integer NOT NULL,
		name text NOT NULL,
		votes text NOT NULL,
		description text NOT NULL,
		UNIQUE (contract)
	);
	`, gdb.p.governorsTableName()))

	return err
}

func (gdb *GovernorDB) drop() error {
	_, err := gdb.p.db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, gdb.p.governorsTableName()))
	return err
}

func (gdb *GovernorDB) ensureExists() error {
	exists, err := gdb.p.checkTableExists(fmt.Sprintf("%s", gdb.p.governorsTableName()))
	if err != nil {
		return err
	}

	if !exists {
		if err = gdb.Create(); err != nil {
			return err
		}

		// TODO: indexes?
	}

	return nil
}

func (gdb *GovernorDB) GetGovernor(contract string) (*govindexer.Governor, error) {
	var gov govindexer.Governor
	contract = strings.ToLower(contract)
	// TODO: rest of fields ...
	err := gdb.p.rdb.QueryRow(fmt.Sprintf(`
	SELECT contract
	FROM %s
	WHERE contract = $1
	`, gdb.p.governorsTableName()), contract).Scan(&gov.Contract)
	if err != nil {
		return nil, err
	}

	return &gov, nil
}

func (gdb *GovernorDB) AddEvent(contract string) error {
	contract = strings.ToLower(contract)
	t := time.Now()
	// TODO: rest of fields !!!
	_, err := gdb.p.db.Exec(fmt.Sprintf(`
    INSERT INTO %s (contract, created_at, updated_at)
    VALUES ($1, $2, $3)
    ON CONFLICT(contract) DO UPDATE SET
        updated_at = excluded.updated_at,
    `, gdb.p.governorsTableName()), contract, t, t)
	return err
}
