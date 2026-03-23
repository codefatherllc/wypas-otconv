# wypas-otconv — Legacy File to DB Seeder

## Build

```bash
go build .
```

## Usage

Seeds MariaDB from legacy Tibia file formats (OTB, XML, OTBM).

### Items

```bash
otconv items seed --otb items.otb --xml items.xml --dsn "user:pass@tcp(host:3306)/db"
```

Parses OTB + XML via `wypas-lib/otb`, inserts into `item_types` table via `wypas-lib/gamedata`.

### Map

```bash
otconv map seed --otbm Map.otbm --spawns Spawns.xml --houses Houses.xml --dsn "user:pass@tcp(host:3306)/db"
```

Parses OTBM + XMLs via `wypas-lib/otb`, inserts into `map_tiles`, `map_spawns`, `map_houses`, `map_towns`, `map_waypoints` tables via `wypas-lib/gamedata`.

## Dependencies

- `wypas-lib/otb` — legacy file parsing (OTB+XML, OTBM+XMLs)
- `wypas-lib/gamedata` — DB types and store functions

## Release

Cross-compiled for 5 targets via GitHub Actions (`release.yml` on `v*` tags).

## License

GPL-2.0

## CI

`.github/workflows/build.yml` on `main`, `.github/workflows/release.yml` on `v*` tags.
