-- +goose Up
-- Read-only platform settings singleton (CONVENTIONS §1, PC-1..PC-5).
-- These 7 rows are locked in v1; there is no PUT/PATCH endpoint.
-- Consumed by GET /platform/settings → E1 foundations screen m3sWh.
CREATE TABLE platform_settings (
    key    text PRIMARY KEY,
    value  text NOT NULL,
    label  text NOT NULL,
    locked boolean NOT NULL DEFAULT true,
    sort   int  NOT NULL DEFAULT 0
);

INSERT INTO platform_settings (key, value, label, locked, sort) VALUES
    ('locale',             'id-ID',             'Bahasa Indonesia',                                true, 1),
    ('timezone',           'Asia/Jakarta',       'Asia/Jakarta · WIB (UTC+7) · tanpa DST',         true, 2),
    ('date_format',        'dd MMM yyyy',        '03 Jun 2026',                                    true, 3),
    ('currency',           'IDR',               'Rupiah (IDR)',                                   true, 4),
    ('version',            '1.0.0',             'v1.0.0 · 2026-06-03',                            true, 5),
    ('stack',              'go-react-postgres', 'Go · React · Postgres',                          true, 6),
    ('legacy_data_source', 'lumen_swp',         'MySQL lumen_swp (read-only, migrated via E9)',   true, 7);

-- +goose Down
DROP TABLE IF EXISTS platform_settings;
