package govdb

import (
	"fmt"
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
