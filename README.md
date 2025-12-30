# shinkrodb

A tool for building and maintaining anime databases by aggregating data from MyAnimeList, AniDB, TVDB, and TMDB.

## Installation

```bash
go install github.com/varoOP/shinkrodb/cmd/shinkrodb@latest
```

Or download from [releases](https://github.com/varoOP/shinkrodb/releases).

## Configuration

Create `$HOME/.config/shinkrodb/config.toml` or use `--config` flag. See `config.toml.example` for reference.

**Required:**
- `mal_client_id` - MyAnimeList API Client ID (or `SHINKRODB_MAL_CLIENT_ID`)
- `tmdb_api_key` - TMDB API Key (or `SHINKRODB_TMDB_API_KEY`)

**Optional:**
- `discord_webhook_url` - Discord webhook for notifications (or `SHINKRODB_DISCORD_WEBHOOK_URL`)
- `anidb_mode` / `tmdb_mode` - Fetch modes: `default`, `missing`, `all`, or `skip`

## Usage

```bash
# Run full database update
shinkrodb run [--anidb=<mode>] [--tmdb=<mode>] [--root-path=<path>]

# Migrate old HTML cache to SQLite
shinkrodb migrate

# Format mapping files
shinkrodb format [--root-path=<path>]

# Generate mapping files
shinkrodb genmap [--root-path=<path>]

# Show version
shinkrodb version
```

## Output Files

- `malid.json` - MAL IDs with titles and release dates
- `malid-anidbid.json` - Adds AniDB IDs (scraped from MAL)
- `malid-anidbid-tvdbid.json` - Adds TVDB IDs (from anime-lists)
- `malid-anidbid-tvdbid-tmdbid.json` - Adds TMDB IDs (from anime-lists + TMDB API)
- `for-shinkro.json` - Optimized for shinkro (duplicates removed)

## Features

- **Caching**: SQLite cache for efficient re-runs
- **Configurable Fetching**: Control which entries are scraped/fetched
- **Notifications**: Discord webhook support for run completion
- **Statistics**: Comprehensive coverage reports

## Acknowledgments

- **TVDB IDs**: Provided by [Anime-Lists/anime-lists](https://github.com/Anime-Lists/anime-lists)
- **TMDB IDs**: Sourced from [Anime-Lists/anime-lists](https://github.com/Anime-Lists/anime-lists) and the [TMDB API](https://developer.themoviedb.org/reference/)
