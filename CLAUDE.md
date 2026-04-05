# wypas-otconv — Legacy File to DB Seeder (v2)

## Build

```bash
go build .
```

## Usage

Seeds MariaDB from legacy OpenTibia file formats (OTB, XML, OTBM).
Targets the **migrations-v2** schema (wypas-proxy/migrations-v2/).

### Items

```bash
otconv items seed --otb items.otb --xml items.xml --dsn "user:pass@tcp(host:3306)/db"
```

Parses OTB + XML via `wypas-lib/otb`, inserts into `item_types` table via `wypas-lib/gamedata`.

### Map

```bash
otconv map seed --otbm Map.otbm --spawns Spawns.xml --houses Houses.xml --dsn "user:pass@tcp(host:3306)/db"
```

Parses OTBM + XMLs via `wypas-lib/otbm`, inserts into:
- `map` — tile data (x, y, z, ground_id, flags, house_id)
- `items` — unified items table (each tile item as a row with `owner_type='map'`, `owner_id=PackPos(x,y,z)`, attributes as JSON)
- `spawns` + `spawn` — spawn point areas and individual creature entries
- `houses` — house definitions
- `towns` — town entry points
- `waypoints` — named teleport waypoints

## v2 Schema Changes (from v1)

- `map_tiles` → `map` (renamed, items blob removed — items live in unified `items` table)
- `map_spawns` → `spawns`, `map_spawn_entries` → `spawn` (renamed, spawn.type is ENUM)
- `map_towns` → `towns`, `map_waypoints` → `waypoints`, `map_houses` → `houses` (renamed)
- NEW: `items` table — each tile item gets its own row with JSON attributes (action_id, unique_id, tele_dest, door_id, depot_id, text, charges, etc.)

## Dependencies

- `wypas-lib/otb` — legacy file parsing (OTB+XML, OTBM+XMLs)
- `wypas-lib/otbm` — OTBM parser, LoadWorld
- `wypas-lib/gamedata` — DB types and store functions

## Release

Cross-compiled for 3 platforms (darwin/arm64, linux/arm64, linux/amd64) via GitHub Actions (`release.yml` on `v*` tags). Binaries + checksums published as GitHub Releases.

## License

GPL-2.0

## CI

- `.github/workflows/build.yml` — `main`, PRs, `workflow_dispatch`
- `.github/workflows/release.yml` — `v*` tags, `workflow_dispatch`
