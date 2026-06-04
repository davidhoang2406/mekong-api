CREATE TABLE symbols (
    symbol      TEXT        NOT NULL,
    asset_class TEXT        NOT NULL,
    exchange    TEXT        NOT NULL,
    first_date  DATE        NOT NULL,
    last_date   DATE        NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (symbol, asset_class)
);
CREATE INDEX idx_symbols_asset_class ON symbols (asset_class);

CREATE TABLE ohlcv_bars (
    symbol      TEXT             NOT NULL,
    asset_class TEXT             NOT NULL,
    exchange    TEXT             NOT NULL,
    time        DATE             NOT NULL,
    open        DOUBLE PRECISION NOT NULL,
    high        DOUBLE PRECISION NOT NULL,
    low         DOUBLE PRECISION NOT NULL,
    close       DOUBLE PRECISION NOT NULL,
    volume      BIGINT           NOT NULL,
    PRIMARY KEY (symbol, time)
);
CREATE INDEX idx_ohlcv_symbol_time ON ohlcv_bars (symbol, time DESC);

CREATE TABLE technical_indicators (
    symbol      TEXT             NOT NULL,
    time        DATE             NOT NULL,
    close       DOUBLE PRECISION NOT NULL,
    sma20       DOUBLE PRECISION,
    sma50       DOUBLE PRECISION,
    sma200      DOUBLE PRECISION,
    rsi14       DOUBLE PRECISION,
    macd        DOUBLE PRECISION,
    macd_signal DOUBLE PRECISION,
    macd_hist   DOUBLE PRECISION,
    bb_upper    DOUBLE PRECISION,
    bb_mid      DOUBLE PRECISION,
    bb_lower    DOUBLE PRECISION,
    PRIMARY KEY (symbol, time)
);
CREATE INDEX idx_indicators_symbol_time ON technical_indicators (symbol, time DESC);

CREATE TABLE digest_entries (
    date        DATE             NOT NULL,
    category    TEXT             NOT NULL,
    rank        INTEGER          NOT NULL,
    symbol      TEXT             NOT NULL,
    exchange    TEXT             NOT NULL,
    asset_class TEXT             NOT NULL,
    open        DOUBLE PRECISION NOT NULL,
    close       DOUBLE PRECISION NOT NULL,
    volume      BIGINT           NOT NULL,
    pct_change  DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (date, category, rank)
);
CREATE INDEX idx_digest_date_category ON digest_entries (date, category);

CREATE TABLE screener_results (
    year          TEXT             NOT NULL,
    week          TEXT             NOT NULL,
    symbol        TEXT             NOT NULL,
    pe_ratio      DOUBLE PRECISION,
    pb_ratio      DOUBLE PRECISION,
    roe           DOUBLE PRECISION,
    eps           DOUBLE PRECISION,
    de_ratio      DOUBLE PRECISION,
    current_ratio DOUBLE PRECISION,
    PRIMARY KEY (year, week, symbol)
);
CREATE INDEX idx_screener_year_week ON screener_results (year, week);
