-- name: ListSitesForCompany :many
-- Cursor page: primary first, then created_at desc, id desc.
-- Keyset cursor on (created_at, id); is_primary DESC is the primary sort but
-- keyset uses the sub-sort (created_at, id) for stable pagination.
SELECT id, client_company_id, name, code, address, geo_lat, geo_lng,
       geofence_radius_m, is_primary, pic_name, phone, status, created_at, updated_at
FROM client_sites
WHERE client_company_id = sqlc.arg(client_company_id)
  AND deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (
        sqlc.narg(cursor_created_at)::timestamptz IS NULL
        OR (created_at, id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::text)
      )
ORDER BY is_primary DESC, created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetSiteByID :one
SELECT id, client_company_id, name, code, address, geo_lat, geo_lng,
       geofence_radius_m, is_primary, pic_name, phone, status, created_at, updated_at
FROM client_sites
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: CreateSite :one
-- Allocates the SWP-SITE id inline from the per-prefix sequence.
INSERT INTO client_sites (id, client_company_id, name, code, address, geo_lat, geo_lng,
                          geofence_radius_m, is_primary, pic_name, phone)
VALUES (
    'SWP-SITE-' || swp_next_id('SITE'),
    sqlc.arg(client_company_id),
    sqlc.arg(name),
    sqlc.narg(code),
    sqlc.arg(address),
    sqlc.narg(geo_lat),
    sqlc.narg(geo_lng),
    sqlc.arg(geofence_radius_m),
    sqlc.arg(is_primary),
    sqlc.narg(pic_name),
    sqlc.narg(phone)
)
RETURNING id, client_company_id, name, code, address, geo_lat, geo_lng,
          geofence_radius_m, is_primary, pic_name, phone, status, created_at, updated_at;

-- name: UpdateSite :one
UPDATE client_sites
SET name              = sqlc.arg(name),
    code              = sqlc.narg(code),
    address           = sqlc.arg(address),
    geo_lat           = sqlc.narg(geo_lat),
    geo_lng           = sqlc.narg(geo_lng),
    geofence_radius_m = sqlc.arg(geofence_radius_m),
    pic_name          = sqlc.narg(pic_name),
    phone             = sqlc.narg(phone),
    updated_at        = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, client_company_id, name, code, address, geo_lat, geo_lng,
          geofence_radius_m, is_primary, pic_name, phone, status, created_at, updated_at;

-- name: DemoteOtherPrimaries :exec
-- Clears is_primary on all other sites of the company when a new primary is set.
-- Call inside the same tx before SetSitePrimary (INV-5).
UPDATE client_sites
SET is_primary = false,
    updated_at = now()
WHERE client_company_id = sqlc.arg(client_company_id)
  AND id <> sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: SetSitePrimary :one
UPDATE client_sites
SET is_primary = true,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, client_company_id, name, code, address, geo_lat, geo_lng,
          geofence_radius_m, is_primary, pic_name, phone, status, created_at, updated_at;

-- name: SetSiteStatus :one
-- Drives :deactivate (status='inactive') and :reactivate (status='active').
UPDATE client_sites
SET status     = sqlc.arg(status),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, client_company_id, name, code, address, geo_lat, geo_lng,
          geofence_radius_m, is_primary, pic_name, phone, status, created_at, updated_at;
