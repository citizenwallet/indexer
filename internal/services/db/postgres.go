package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/big"
	"regexp"
	"strings"
	"sync"

	"github.com/citizenwallet/indexer/pkg/indexer"
	_ "github.com/lib/pq"
)

type PostgresDB struct {
	chainID *big.Int
	mu      sync.Mutex
	db      *sql.DB
	rdb     *sql.DB

	EventDB     *EventDB
	SponsorDB   *SponsorDB
	TransferDB  map[string]*TransferDB
	PushTokenDB map[string]*PushTokenDB
}

func NewPostgresDB(chainID *big.Int, username, password, name, host, rhost, secret string) (*PostgresDB, error) {
	connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=5432 sslmode=disable", username, password, name, host)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	rconnStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=5432 sslmode=disable", username, password, name, rhost)
	rdb, err := sql.Open("postgres", rconnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	evname := chainID.String()

	eventDB, err := NewEventDB(db, db, evname)
	if err != nil {
		return nil, err
	}

	sponsorDB, err := NewSponsorDB(db, db, evname, secret)
	if err != nil {
		return nil, err
	}

	pdb := &PostgresDB{
		chainID:   chainID,
		db:        db,
		rdb:       rdb,
		EventDB:   eventDB,
		SponsorDB: sponsorDB,
	}

	// check if db exists before opening, since we use rwc mode
	exists, err := pdb.EventTableExists(evname)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("event table does not exist")
	}

	// check if db exists before opening, since we use rwc mode
	exists, err = pdb.SponsorTableExists(evname)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, fmt.Errorf("sponsor table does not exist")
	}

	txdb := map[string]*TransferDB{}
	ptdb := map[string]*PushTokenDB{}

	evs, err := eventDB.GetEvents()
	if err != nil {
		return nil, err
	}

	for _, ev := range evs {
		name, err := pdb.TableNameSuffix(ev.Contract)
		if err != nil {
			return nil, err
		}

		log.Default().Println("creating transfer db for: ", name)

		txdb[name], err = NewTransferDB(db, db, name)
		if err != nil {
			return nil, err
		}

		// check if db exists before opening, since we use rwc mode
		exists, err := pdb.TransferTableExists(name)
		if err != nil {
			return nil, err
		}

		if !exists {
			// create table
			err = txdb[name].CreateTransferTable()
			if err != nil {
				return nil, err
			}

			// create indexes
			err = txdb[name].CreateTransferTableIndexes()
			if err != nil {
				return nil, err
			}
		}

		log.Default().Println("creating push token db for: ", name)

		ptdb[name], err = NewPushTokenDB(db, db, name)
		if err != nil {
			return nil, err
		}

		// check if db exists before opening, since we use rwc mode
		exists, err = pdb.PushTokenTableExists(name)
		if err != nil {
			return nil, err
		}

		if !exists {
			// create table
			err = ptdb[name].CreatePushTable()
			if err != nil {
				return nil, err
			}

			// create indexes
			err = ptdb[name].CreatePushTableIndexes()
			if err != nil {
				return nil, err
			}
		}
	}

	pdb.TransferDB = txdb
	pdb.PushTokenDB = ptdb

	return pdb, nil
}

// EventTableExists checks if a table exists in the database
func (db *PostgresDB) EventTableExists(suffix string) (bool, error) {
	var exists bool
	err := db.db.QueryRow(fmt.Sprintf(`
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public'
        AND table_name = 't_events_%s'
    );
    `, suffix)).Scan(&exists)
	if err != nil {
		return false, err
	}

	return true, nil
}

// SponsorTableExists checks if a table exists in the database
func (db *PostgresDB) SponsorTableExists(suffix string) (bool, error) {
	var exists bool
	err := db.db.QueryRow(fmt.Sprintf(`
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public'
        AND table_name = 't_sponsors_%s'
    );
    `, suffix)).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// TransferTableExists checks if a table exists in the database
func (db *PostgresDB) TransferTableExists(suffix string) (bool, error) {
	var exists bool
	err := db.db.QueryRow(fmt.Sprintf(`
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public'
        AND table_name = 't_transfers_%s'
    );
    `, suffix)).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// PushTokenTableExists checks if a table exists in the database
func (db *PostgresDB) PushTokenTableExists(suffix string) (bool, error) {
	var exists bool
	err := db.db.QueryRow(fmt.Sprintf(`
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_schema = 'public'
        AND table_name = 't_push_token_%s'
    );
    `, suffix)).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// TableNameSuffix returns the name of the transfer db for the given contract
func (d *PostgresDB) TableNameSuffix(contract string) (string, error) {
	re := regexp.MustCompile("^0x[0-9a-fA-F]{40}$")

	suffix := fmt.Sprintf("%v_%s", d.chainID, strings.ToLower(contract))

	if !re.MatchString(contract) {
		return suffix, errors.New("bad contract address")
	}

	return suffix, nil
}

// GetTransferDB returns true if the transfer db for the given contract exists, returns the db if it exists
func (d *PostgresDB) GetTransferDB(contract string) (*TransferDB, bool) {
	name, err := d.TableNameSuffix(contract)
	if err != nil {
		return nil, false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	txdb, ok := d.TransferDB[name]
	if !ok {
		return nil, false
	}
	return txdb, true
}

// GetPushTokenDB returns true if the push token db for the given contract exists, returns the db if it exists
func (d *PostgresDB) GetPushTokenDB(contract string) (*PushTokenDB, bool) {
	name, err := d.TableNameSuffix(contract)
	if err != nil {
		return nil, false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	ptdb, ok := d.PushTokenDB[name]
	if !ok {
		return nil, false
	}
	return ptdb, true
}

// AddTransferDB adds a new transfer db for the given contract
func (d *PostgresDB) AddTransferDB(contract string) (*TransferDB, error) {
	name, err := d.TableNameSuffix(contract)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if txdb, ok := d.TransferDB[name]; ok {
		return txdb, nil
	}
	txdb, err := NewTransferDB(d.db, d.rdb, name)
	if err != nil {
		return nil, err
	}
	d.TransferDB[name] = txdb
	return txdb, nil
}

// AddPushTokenDB adds a new push token db for the given contract
func (d *PostgresDB) AddPushTokenDB(contract string) (*PushTokenDB, error) {
	name, err := d.TableNameSuffix(contract)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if ptdb, ok := d.PushTokenDB[name]; ok {
		return ptdb, nil
	}
	ptdb, err := NewPushTokenDB(d.db, d.rdb, name)
	if err != nil {
		return nil, err
	}
	d.PushTokenDB[name] = ptdb
	return ptdb, nil
}

// Close closes the db and all its transfer and push dbs
func (d *PostgresDB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, txdb := range d.TransferDB {
		err := txdb.Close()
		if err != nil {
			return err
		}

		delete(d.TransferDB, i)
	}

	for i, ptdb := range d.PushTokenDB {
		err := ptdb.Close()
		if err != nil {
			return err
		}

		delete(d.PushTokenDB, i)
	}

	err := d.SponsorDB.Close()
	if err != nil {
		return err
	}

	return d.EventDB.Close()
}

// Migrate to Sqlite db
func (d *PostgresDB) Migrate(sqdb *DB, token, paymaster string, txBatchSize int) error {
	log.Default().Println("starting migration...")

	// instantiate tables
	name, err := d.TableNameSuffix(token)
	if err != nil {
		return err
	}

	pushDB, exists := sqdb.GetPushTokenDB(token)
	if !exists {
		p, err := NewPushTokenDB(sqdb.db, sqdb.rdb, name)
		if err != nil {
			return err
		}

		err = p.CreatePushTable()
		if err != nil {
			return err
		}

		pushDB = p
	}

	txdb, exists := sqdb.GetTransferDB(token)
	if !exists {
		t, err := NewTransferDB(sqdb.db, sqdb.rdb, name)
		if err != nil {
			return err
		}

		err = t.CreateTransferTable()
		if err != nil {
			return err
		}

		txdb = t
	}

	// fetch all events
	evs, err := d.EventDB.GetEvents()
	if err != nil {
		return err
	}

	log.Default().Println("migrating ", len(evs), " events")

	// add them to the new db
	for _, ev := range evs {
		if ev.Contract != token {
			continue
		}

		log.Default().Println("migrating event: ", ev.Contract, ev.Name, ev.Symbol)

		err := sqdb.EventDB.AddEvent(ev.Contract, ev.State, ev.StartBlock, ev.LastBlock, ev.Standard, ev.Name, ev.Symbol, ev.Decimals)
		if err != nil {
			return err
		}

		if paymaster != "" {
			// migrate paymaster if provided

			// fetch sponsor
			sponsor, err := d.SponsorDB.GetSponsor(paymaster)
			if err != nil {
				return err
			}

			log.Default().Println("migrating sponsor: ", sponsor.Contract)

			// add sponsor
			err = sqdb.SponsorDB.AddSponsor(sponsor)
			if err != nil {
				return err
			}
		}

		log.Default().Println("migrating push tokens")

		rows, err := countRows(d.rdb, fmt.Sprintf("t_push_token_%s", name))
		if err != nil {
			return err
		}

		// migrate all push tokens
		batchSize := 100
		offset := 0
		for {
			log.Default().Println(offset, "/", rows, "...")
			rows, err := d.rdb.Query(fmt.Sprintf("SELECT token, account FROM t_push_token_%s ORDER BY token LIMIT $1 OFFSET $2", name), batchSize, offset)
			if err != nil {
				log.Fatal(err)
			}

			var count int
			for rows.Next() {
				// Process each row here.
				var p indexer.PushToken
				err := rows.Scan(&p.Token, &p.Account)
				if err != nil {
					return err
				}

				err = pushDB.AddToken(&p)
				if err != nil {
					return err
				}

				count++
			}

			log.Default().Println("migrated ", count, " push tokens")

			if count < batchSize {
				// If we fetched fewer rows than the batch size, we've fetched all rows.
				break
			}

			// Increment the offset by batchSize for the next iteration.
			offset += batchSize
		}
		log.Default().Println(rows, "/", rows)

		log.Default().Println("migrating transfers")

		rows, err = countRows(d.rdb, fmt.Sprintf("t_transfers_%s", name))
		if err != nil {
			return err
		}

		// migrate all transfers
		offset = 0
		for {
			log.Default().Println(offset, "/", rows, "...")
			rows, err := d.rdb.Query(fmt.Sprintf(`
				SELECT hash, tx_hash, token_id, created_at, from_to_addr, from_addr, to_addr, nonce, value, data, status
				FROM t_transfers_%s ORDER BY created_at LIMIT $1  OFFSET $2
			`, name), txBatchSize, offset)
			if err != nil {
				log.Fatal(err)
			}

			var count int
			for rows.Next() {
				// Process each row here.
				var transfer indexer.Transfer
				var value string

				err := rows.Scan(&transfer.Hash, &transfer.TxHash, &transfer.TokenID, &transfer.CreatedAt, &transfer.FromTo, &transfer.From, &transfer.To, &transfer.Nonce, &value, &transfer.Data, &transfer.Status)
				if err != nil {
					return err
				}

				transfer.Value = new(big.Int)
				transfer.Value.SetString(value, 10)

				err = txdb.AddTransfer(&transfer)
				if err != nil {
					return err
				}

				count++
			}
			log.Default().Println(rows, "/", rows)

			log.Default().Println("migrated ", count, " transfer events")

			if count < txBatchSize {
				// If we fetched fewer rows than the batch size, we've fetched all rows.
				break
			}

			// Increment the offset by batchSize for the next iteration.
			offset += txBatchSize
		}
	}

	return nil
}

func countRows(db *sql.DB, tableName string) (int, error) {
	var count int

	// Prepare the SQL statement
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	err := db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
