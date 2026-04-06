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
- `map` — tile structure (packed pos PK `(x<<20)|(y<<4)|z`, flags, house_id). No ground_id.
- `map_items` — immutable template items (tile_pos FK, slot=0 is ground, slot=1+ are items in stackpos order, JSON attributes)
- `spawns` + `spawn` — spawn point areas and individual creature entries
- `houses` — house definitions
- `towns` — town entry points
- `waypoints` — named teleport waypoints

## v2 Schema

- `map`: packed `pos` BIGINT PK `(x<<20)|(y<<4)|z`, `flags`, `house_id`. No ground_id column.
- `map_items`: `tile_pos` FK to map.pos, `slot=0` = ground item, `slot=1+` = stackpos. Immutable at runtime.
- `items`: game runtime state. `parent_type` ENUM('world','player','container','depot','market') + `parent_id`.
- Position packing: `(x<<20)|(y<<4)|z` — z: 4 bits, y: 16 bits, x: 16 bits.

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
