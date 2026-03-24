# wypas-otconv

Database seeder for Wypas — imports legacy OpenTibia data files (items.otb, items.xml, Map.otbm, Spawns.xml, Houses.xml) into MariaDB tables.

## Build

```bash
go build .
```

## Usage

```bash
# Seed item types
otconv items seed --otb items.otb --xml items.xml --dsn "user:pass@tcp(host:3306)/dbname"

# Seed map data (tiles, spawns, houses, towns, waypoints)
otconv map seed --otbm Map.otbm --spawns Spawns.xml --houses Houses.xml --dsn "user:pass@tcp(host:3306)/dbname"
```

## License

GPL-2.0
