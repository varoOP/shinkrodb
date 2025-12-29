package app

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/varoOP/shinkrodb/internal/config"
	"github.com/varoOP/shinkrodb/internal/database"
	"github.com/varoOP/shinkrodb/internal/dedupe"
	"github.com/varoOP/shinkrodb/internal/domain"
	"github.com/varoOP/shinkrodb/internal/format"
	"github.com/varoOP/shinkrodb/internal/logger"
	"github.com/varoOP/shinkrodb/internal/mal"
	"github.com/varoOP/shinkrodb/internal/repository"
	"github.com/varoOP/shinkrodb/internal/tmdb"
	"github.com/varoOP/shinkrodb/internal/tvdb"
)

// App represents the main application with all dependencies initialized
type App struct {
	log           zerolog.Logger
	config        *domain.Config
	paths         *domain.Paths
	animeRepo     domain.AnimeRepository
	mappingRepo   domain.MappingRepository
	malService    mal.Service
	tmdbService   tmdb.Service
	tvdbService   tvdb.Service
	dedupeService dedupe.Service
}

// NewApp creates a new application instance with all dependencies initialized
func NewApp() (*App, error) {
	// Initialize logger
	log := logger.NewLogger()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize paths (will be set properly when root path is known)
	paths := domain.NewPaths(".")

	// Initialize repositories
	fileRepo := repository.NewFileRepository(log)
	var animeRepo domain.AnimeRepository = fileRepo
	var mappingRepo domain.MappingRepository = fileRepo

	// Initialize services
	malService := mal.NewService(log, cfg, animeRepo, paths.MalIDPath, paths.AniDBPath)
	tmdbService := tmdb.NewService(log, cfg, animeRepo, mappingRepo, paths)
	tvdbService := tvdb.NewService(log, animeRepo, mappingRepo, paths)
	dedupeService := dedupe.NewService(log, animeRepo)

	return &App{
		log:           log,
		config:        cfg,
		paths:         paths,
		animeRepo:     animeRepo,
		mappingRepo:   mappingRepo,
		malService:    malService,
		tmdbService:   tmdbService,
		tvdbService:   tvdbService,
		dedupeService: dedupeService,
	}, nil
}

// Run executes the full database update process
func (a *App) Run(rootPath string) error {
	ctx := context.Background()

	// Update paths with actual root path
	a.paths = domain.NewPaths(rootPath)

	// Update services with new paths
	a.malService = mal.NewService(a.log, a.config, a.animeRepo, a.paths.MalIDPath, a.paths.AniDBPath)
	a.tmdbService = tmdb.NewService(a.log, a.config, a.animeRepo, a.mappingRepo, a.paths)
	a.tvdbService = tvdb.NewService(a.log, a.animeRepo, a.mappingRepo, a.paths)

	// Get MAL IDs
	if err := a.malService.GetAnimeIDs(ctx); err != nil {
		return fmt.Errorf("failed to get MAL IDs: %w", err)
	}

	// Initialize database and cache repository
	db, err := database.NewDB(rootPath, a.log)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	cacheRepo := database.NewCacheRepo(a.log, db)

	// Scrape MAL for AniDB IDs (cache invalidation happens implicitly - only entries < 1 year old are used)
	if err := a.malService.ScrapeAniDBIDs(ctx, cacheRepo); err != nil {
		return fmt.Errorf("failed to scrape MAL: %w", err)
	}

	// Get TVDB IDs and update mapping
	if err := a.tvdbService.GetTvdbIDs(ctx, rootPath); err != nil {
		return fmt.Errorf("failed to get TVDB IDs: %w", err)
	}

	// Get TMDB IDs
	if err := a.tmdbService.GetTmdbIds(ctx, rootPath); err != nil {
		return fmt.Errorf("failed to get TMDB IDs: %w", err)
	}

	// Check for duplicates
	animeList, err := a.animeRepo.Get(ctx, a.paths.TMDBPath)
	if err != nil {
		return fmt.Errorf("failed to get anime list: %w", err)
	}

	dupeCount, deduped, err := a.dedupeService.CheckDupes(ctx, animeList)
	if err != nil {
		return fmt.Errorf("failed to check dupes: %w", err)
	}

	a.log.Info().Int("dupe_count", dupeCount).Msg("Duplicate check complete")

	// Store deduped list
	if err := a.animeRepo.Store(ctx, a.paths.ShinkroPath, deduped); err != nil {
		return fmt.Errorf("failed to store deduped anime: %w", err)
	}

	return nil
}

// GenerateMappings generates mapping files from master files
func (a *App) GenerateMappings(rootPath string) error {
	ctx := context.Background()

	// Generate TMDB mapping
	am, err := a.mappingRepo.GetTMDBMaster(ctx, filepath.Join(rootPath, "tmdb-mal-master.yaml"))
	if err != nil {
		return fmt.Errorf("failed to get TMDB master: %w", err)
	}

	// Create filtered mapping (only entries with TMDB IDs)
	filtered := &domain.AnimeMovies{}
	for _, movie := range am.AnimeMovie {
		if movie.TMDBID != 0 {
			filtered.AnimeMovie = append(filtered.AnimeMovie, movie)
		}
	}

	if err := a.mappingRepo.StoreTMDBMaster(ctx, filepath.Join(rootPath, "tmdb-mal.yaml"), filtered); err != nil {
		return fmt.Errorf("failed to create TMDB mapping: %w", err)
	}

	// Generate TVDB mapping
	tvdb, err := a.mappingRepo.GetTVDBMaster(ctx, filepath.Join(rootPath, "tvdb-mal-master.yaml"))
	if err != nil {
		return fmt.Errorf("failed to get TVDB master: %w", err)
	}

	// Create filtered mapping (only entries with TVDB IDs)
	filteredTVDB := &domain.TVDBMap{}
	for _, anime := range tvdb.Anime {
		if anime.Tvdbid != 0 {
			filteredTVDB.Anime = append(filteredTVDB.Anime, anime)
		}
	}

	if err := a.mappingRepo.StoreTVDBMaster(ctx, filepath.Join(rootPath, "tvdb-mal.yaml"), filteredTVDB); err != nil {
		return fmt.Errorf("failed to create TVDB mapping: %w", err)
	}

	return nil
}

// FormatFiles formats the YAML mapping files
func (a *App) FormatFiles(rootPath string) error {
	if err := format.FormatTMDB(rootPath, a.mappingRepo); err != nil {
		return fmt.Errorf("failed to format TMDB: %w", err)
	}

	if err := format.FormatTVDB(rootPath, a.mappingRepo); err != nil {
		return fmt.Errorf("failed to format TVDB: %w", err)
	}

	return nil
}
