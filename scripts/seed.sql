CREATE TABLE IF NOT EXISTS t_events_84532(
    contract text NOT NULL,
    state text NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    start_block integer NOT NULL,
    last_block integer NOT NULL,
    standard text NOT NULL,
    name text NOT NULL,
    symbol text NOT NULL,
    decimals integer NOT NULL DEFAULT 6,
    UNIQUE (contract, standard)
);

CREATE INDEX IF NOT EXISTS idx_events_84532_state ON t_events_84532(state);

CREATE INDEX IF NOT EXISTS idx_events_84532_address_signature ON t_events_84532(contract, standard);

CREATE INDEX IF NOT EXISTS idx_events_84532_address_signature_state ON t_events_84532(contract, standard, state);

INSERT INTO t_events_84532(contract, state, created_at, updated_at, start_block, last_block, standard, name, symbol, decimals)
    VALUES ('0x9b1a0D2951b11Ac26A6cBbd5aEf2c4cb014b3B6e', 'indexed', '2024-03-08 12:02:25.862592+01:00', '2024-03-08 12:02:25.862592+01:00', '32945384', '32945384', 'ERC20', 'ETHGlobal London Token', 'ETHLDN', 6);

CREATE TABLE IF NOT EXISTS t_sponsors_84532(
    contract text NOT NULL PRIMARY KEY,
    pk text NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO t_sponsors_84532(contract, pk, created_at, updated_at)
    VALUES ('0x389182aCCeE26D953d5188BF4b92c49339DcC9FC', 'HktJpk6mLNkUwEdHwe8CjXDytxy9yuJCbWwB_Vmh8ei3DD_HdmXl44lfiGRtxwhUIMnU8rEzxHeGeWsWCyG0_4IkcoGlY7H5BSJiKpTkG2c=', '2024-03-08 12:02:25.862592+01:00', '2024-03-08 12:02:25.862592+01:00');

