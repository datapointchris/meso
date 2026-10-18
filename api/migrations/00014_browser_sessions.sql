-- +goose Up
-- The tables gobrowserlogin's postgresstore keeps browser logins in, copied
-- from postgresstore.Schema. The API refuses to start without them.

-- A login a browser has started and not finished. state_sha256 is the SHA-256
-- of the OAuth state, and a callback deletes the row as it reads it.
CREATE TABLE browser_login_attempts (
    state_sha256 BYTEA PRIMARY KEY,
    nonce TEXT NOT NULL,
    code_verifier TEXT NOT NULL,
    return_to TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX browser_login_attempts_expires_at ON browser_login_attempts (expires_at);

-- One browser's login. cookie_sha256 is the SHA-256 of the session cookie's
-- value, so a copy of this table names no cookie a reader could present.
CREATE TABLE browser_sessions (
    cookie_sha256 BYTEA PRIMARY KEY,
    identity JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    seen_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX browser_sessions_expires_at ON browser_sessions (expires_at);

-- +goose Down
DROP TABLE browser_sessions;
DROP TABLE browser_login_attempts;
