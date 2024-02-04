package govdb

import "fmt"

/**
 * @dev Emitted when a proposal is created.
 */
//event ProposalCreated(
//	uint256 proposalId,
//	address proposer,
//	address[] targets,
//	uint256[] values,
//	string[] signatures,
//	bytes[] calldatas,
//	uint256 voteStart,
//	uint256 voteEnd,
//	string description
//);

//     enum ProposalState {
//        Pending,
//        Active,
//        Canceled,
//        Defeated,
//        Succeeded,
//        Queued,
//        Expired,
//        Executed
//    }

type ProposalDB struct {
	p *DB
}

func (gdb *ProposalDB) Create() error {
	_, err := gdb.p.db.Exec(fmt.Sprintf(`
	CREATE TABLE %s(
	    governor varchar(40) NOT NULL,
	    proposal_id varchar(64) NOT NULL,
	    proposer varchar(40) NOT NULL,
	    state varchar(10) NOT NULL,
	    
	    targets varchar(40) ARRAY,
	    valuez varchar(64) ARRAY,
	    signatures text ARRAY,
	    calldatas text ARRAY,
	    
		created_at timestamp NOT NULL,
		updated_at timestamp NOT NULL,

	    vote_start timestamp NOT NULL,
		vote_end timestamp NOT NULL,

		name text NOT NULL,
		description text NOT NULL,
		UNIQUE (governor,proposal_id)
	);
	`, gdb.p.proposalsTableName()))

	return err
}

func (gdb *ProposalDB) drop() error {
	_, err := gdb.p.db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, gdb.p.proposalsTableName()))
	return err
}

func (pdb *ProposalDB) ensureExists() error {
	exists, err := pdb.p.checkTableExists(fmt.Sprintf("%s", pdb.p.proposalsTableName()))
	if err != nil {
		return err
	}

	if !exists {
		if err = pdb.Create(); err != nil {
			return err
		}

		// TODO: indexes?
	}

	return nil
}
