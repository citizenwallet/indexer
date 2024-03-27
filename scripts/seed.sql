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
    VALUES ('<token-contract>', 'indexed', '2024-03-08 12:02:25.862592+01:00', '2024-03-08 12:02:25.862592+01:00', '32945384', '32945384', 'ERC20', '<token-name>', '<token-symbol>', 6);

CREATE TABLE IF NOT EXISTS t_sponsors_84532(
    contract text NOT NULL PRIMARY KEY,
    pk text NOT NULL,
    created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO t_sponsors_84532(contract, pk, created_at, updated_at)
    VALUES ('<paymaster-contract>', '<encrypted-key>', '2024-03-08 12:02:25.862592+01:00', '2024-03-08 12:02:25.862592+01:00');

