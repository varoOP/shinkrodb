package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/varoOP/shinkrodb/internal/domain"
	"github.com/varoOP/shinkrodb/pkg/animelist"
)

type Service interface {
	GetTmdbIds(ctx context.Context, rootPath string, cacheRepo domain.CacheRepo) error
}

type service struct {
	log         zerolog.Logger
	config      *domain.Config
	animeRepo   domain.AnimeRepository
	mappingRepo domain.MappingRepository
	paths       *domain.Paths
}

type TMDBAPIResponse struct {
	Page    int `json:"page"`
	Results []struct {
		Adult            bool    `json:"adult"`
		BackdropPath     string  `json:"backdrop_path"`
		GenreIds         []int   `json:"genre_ids"`
		ID               int     `json:"id"`
		OriginalLanguage string  `json:"original_language"`
		OriginalTitle    string  `json:"original_title"`
		Overview         string  `json:"overview"`
		Popularity       float64 `json:"popularity"`
		PosterPath       string  `json:"poster_path"`
		ReleaseDate      string  `json:"release_date"`
		Title            string  `json:"title"`
		Video            bool    `json:"video"`
		VoteAverage      float64 `json:"vote_average"`
		VoteCount        int     `json:"vote_count"`
	} `json:"results"`
	TotalPages   int `json:"total_pages"`
	TotalResults int `json:"total_results"`
}

func NewService(log zerolog.Logger, config *domain.Config, animeRepo domain.AnimeRepository, mappingRepo domain.MappingRepository, paths *domain.Paths) Service {
	return &service{
		log:         log.With().Str("module", "tmdb").Logger(),
		config:      config,
		animeRepo:   animeRepo,
		mappingRepo: mappingRepo,
		paths:       paths,
	}
}

func (s *service) GetTmdbIds(ctx context.Context, rootPath string, cacheRepo domain.CacheRepo) error {
	a, err := s.animeRepo.Get(ctx, s.paths.TVDBPath)
	if err != nil {
		return errors.Wrap(err, "failed to get anime list")
	}

	// Get cached TMDB IDs
	cachedTmdbIDs := make(map[int]int)
	if cacheRepo != nil {
		tmdbMap, err := cacheRepo.GetTMDBIDs(ctx)
		if err == nil {
			cachedTmdbIDs = tmdbMap
			// Update anime list with cached TMDB IDs
			for i := range a {
				if tmdbID, found := cachedTmdbIDs[a[i].MalID]; found && tmdbID > 0 {
					a[i].TmdbID = tmdbID
				}
			}
		}
	}

	// Filter movies to fetch based on configured TMDB mode
	toFetch := s.filterMoviesToFetch(a, cachedTmdbIDs)

	if len(toFetch) == 0 {
		s.log.Info().Msg("All movies already cached, skipping TMDB lookups")
		// Still store the updated list with cached TMDB IDs
		if err := s.animeRepo.Store(ctx, s.paths.TMDBPath, a); err != nil {
			return errors.Wrap(err, "failed to store TMDB IDs")
		}
		// Still update master files
		return s.updateMasterFiles(ctx, rootPath, a)
	}

	// Load anime-list.xml from current directory (./) instead of root-path
	al, err := animelist.NewAnimeList(ctx, ".")
	if err != nil {
		s.log.Warn().Err(err).Msg("failed to load anime-list.xml, will use TMDB API only")
		al = nil
	}

	u := s.buildUrl(s.config.TmdbApiKey)
	am := &domain.AnimeMovies{}
	noTmdbTotal := 0
	withTmdbTotal := 0
	fromAnimeListTotal := 0
	totalMovies := 0

	// Build map for O(1) MAL ID lookup
	malIDToIndex := make(map[int]int, len(a))
	for i := range a {
		malIDToIndex[a[i].MalID] = i
	}

	for _, anime := range toFetch {
		totalMovies++
		matched := false

		// First, try to get TMDB ID from anime-list.xml if we have an AniDB ID
		if al != nil && anime.AnidbID > 0 {
			if tmdbID := al.GetTmdbID(anime.AnidbID); tmdbID > 0 {
				// Found in anime-list.xml
				if i, found := malIDToIndex[anime.MalID]; found {
					a[i].TmdbID = tmdbID
					withTmdbTotal++
					fromAnimeListTotal++
					matched = true
					s.log.Debug().
						Str("title", anime.MainTitle).
						Int("tmdb_id", tmdbID).
						Int("anidb_id", anime.AnidbID).
						Msg("TMDBID found in anime-list.xml")

					// Update cache immediately when TMDB ID is found
					if cacheRepo != nil {
						now := time.Now().Format(time.RFC3339)
						entry := &domain.CacheEntry{
							MalID:       anime.MalID,
							AnidbID:     a[i].AnidbID,
							TmdbID:      tmdbID,
							URL:         fmt.Sprintf("https://myanimelist.net/anime/%d", anime.MalID),
							CachedAt:    now,
							LastUsed:    now,
							HadAniDBID:  a[i].AnidbID > 0,
							ReleaseDate: anime.ReleaseDate,
							Type:        anime.Type,
						}

						if err := cacheRepo.UpsertEntry(ctx, entry); err != nil {
							s.log.Warn().Err(err).Int("mal_id", anime.MalID).Msg("failed to update cache")
						} else {
							s.log.Debug().Int("mal_id", anime.MalID).Int("tmdb_id", tmdbID).Msg("Updated cache")
						}
					}
				}
			}
		}

		// If not found in anime-list.xml, fall back to TMDB API
		if !matched {
			target := *u
			query := target.Query()
			if anime.EnglishTitle != "" {
				query.Add("query", anime.EnglishTitle)
			} else {
				query.Add("query", anime.MainTitle)
			}

			if anime.ReleaseDate == "" {
				noTmdbTotal++
				s.log.Debug().Str("title", anime.MainTitle).Msg("does not have a release date")
				am.Add(anime.MainTitle, 0, anime.MalID)
				continue
			}

			year := s.getYear(anime.ReleaseDate)
			query.Add("year", year)
			target.RawQuery = query.Encode()

			tmdb, err := s.searchTMDB(ctx, target.String())
			if err != nil {
				s.log.Warn().Err(err).Str("title", anime.MainTitle).Msg("failed to search TMDB")
				noTmdbTotal++
				am.Add(anime.MainTitle, 0, anime.MalID)
				continue
			}

			// Try old matching logic first (exact date match OR single result)
			var tmdbID int
			for _, result := range tmdb.Results {
				if result.ReleaseDate == anime.ReleaseDate || tmdb.TotalResults == 1 {
					tmdbID = result.ID
					matched = true
					s.log.Debug().Str("title", anime.MainTitle).Int("tmdb_id", result.ID).Msg("TMDBID added from API (old logic)")
					break
				}
			}

			// Log warning only if old logic failed and we have multiple results
			if !matched && tmdb.TotalResults > 1 {
				s.log.Warn().
					Str("title", anime.MainTitle).
					Str("mal_date", anime.ReleaseDate).
					Int("total_results", tmdb.TotalResults).
					Msg("TMDB date does not match MAL date and has multiple results")
			}

			// If old logic failed, try new confidence score method
			if !matched {
				s.log.Trace().
					Str("title", anime.MainTitle).
					Int("mal_id", anime.MalID).
					Msg("Old matching logic failed, trying confidence score method")

				bestMatch := s.findBestMatch(anime, tmdb.Results)
				const minConfidenceScore = 50.0 // Minimum score to accept a match

				// If still no match, try fallback searches with synonyms/Japanese titles
				if bestMatch == nil || bestMatch.Score < minConfidenceScore {
					if tmdb.TotalResults == 0 || bestMatch == nil {
						s.log.Trace().
							Str("title", anime.MainTitle).
							Int("mal_id", anime.MalID).
							Msg("No results from confidence score, trying fallback searches with synonyms/Japanese titles")
					} else {
						s.log.Trace().
							Str("title", anime.MainTitle).
							Int("mal_id", anime.MalID).
							Float64("score", bestMatch.Score).
							Msg("Low confidence score, trying fallback searches")
					}

					// Try fallback searches
					fallbackMatch := s.tryFallbackSearches(ctx, anime, u, year)
					if fallbackMatch != nil && (bestMatch == nil || fallbackMatch.Score > bestMatch.Score) {
						bestMatch = fallbackMatch
						s.log.Trace().
							Str("title", anime.MainTitle).
							Int("tmdb_id", bestMatch.ID).
							Float64("score", bestMatch.Score).
							Msg("Found match via fallback search")
					}
				}

				if bestMatch != nil && bestMatch.Score >= minConfidenceScore {
					tmdbID = bestMatch.ID
					matched = true
					s.log.Info().
						Str("title", anime.MainTitle).
						Int("mal_id", anime.MalID).
						Int("tmdb_id", bestMatch.ID).
						Float64("match_score", bestMatch.Score).
						Msg("TMDB ID found (confidence score method)")
				}
			}

			// Update anime list and cache if matched
			if matched && tmdbID > 0 {
				// O(1) lookup using map
				if i, found := malIDToIndex[anime.MalID]; found {
					a[i].TmdbID = tmdbID
					withTmdbTotal++

					// Update cache immediately when TMDB ID is found
					if cacheRepo != nil {
						now := time.Now().Format(time.RFC3339)
						entry := &domain.CacheEntry{
							MalID:       anime.MalID,
							AnidbID:     a[i].AnidbID,
							TmdbID:      tmdbID,
							URL:         fmt.Sprintf("https://myanimelist.net/anime/%d", anime.MalID),
							CachedAt:    now,
							LastUsed:    now,
							HadAniDBID:  a[i].AnidbID > 0,
							ReleaseDate: anime.ReleaseDate,
							Type:        anime.Type,
						}

						if err := cacheRepo.UpsertEntry(ctx, entry); err != nil {
							s.log.Warn().Err(err).Int("mal_id", anime.MalID).Msg("failed to update cache")
						}
					}
				}
			}

			if !matched {
				noTmdbTotal++
				am.Add(anime.MainTitle, 0, anime.MalID)
				s.log.Warn().
					Str("title", anime.MainTitle).
					Int("mal_id", anime.MalID).
					Str("english_title", anime.EnglishTitle).
					Str("release_date", anime.ReleaseDate).
					Msg("No TMDB ID found")
			}
		}
	}

	if err := s.animeRepo.Store(ctx, s.paths.TMDBPath, a); err != nil {
		return errors.Wrap(err, "failed to store TMDB IDs")
	}

	s.log.Info().
		Int("total_movies", totalMovies).
		Int("with_tmdbid", withTmdbTotal).
		Int("from_anime_list", fromAnimeListTotal).
		Int("from_api", withTmdbTotal-fromAnimeListTotal).
		Int("without_tmdbid", noTmdbTotal).
		Msg("TMDB ID mapping complete")

	if err := s.animeRepo.Store(ctx, s.paths.TMDBPath, a); err != nil {
		return errors.Wrap(err, "failed to store TMDB IDs")
	}

	return s.updateMasterFiles(ctx, rootPath, a)
}

// filterMoviesToFetch filters movies based on configured TMDB mode
func (s *service) filterMoviesToFetch(animeList []domain.Anime, cachedTmdbIDs map[int]int) []domain.Anime {
	// Skip fetching if mode is set to skip
	if s.config.TMDBMode == domain.FetchModeSkip {
		return []domain.Anime{}
	}

	toFetch := []domain.Anime{}

	for _, anime := range animeList {
		if anime.Type != "movie" {
			continue
		}

		shouldFetch := false

		switch s.config.TMDBMode {
		case domain.FetchModeAll:
			// Fetch everything, even if already has TMDB ID in cache
			shouldFetch = true

		case domain.FetchModeMissing:
			// Fetch all movies without TMDB ID (no filters)
			// Skip if already cached with TMDB ID
			if _, found := cachedTmdbIDs[anime.MalID]; found && anime.TmdbID > 0 {
				continue
			}
			shouldFetch = true

		case domain.FetchModeDefault:
			// Default: only fetch movies without TMDB ID
			// Skip if already cached with TMDB ID
			if _, found := cachedTmdbIDs[anime.MalID]; found && anime.TmdbID > 0 {
				continue
			}
			shouldFetch = true
		}

		if shouldFetch {
			toFetch = append(toFetch, anime)
		}
	}

	return toFetch
}

// updateMasterFiles updates the TMDB master mapping files
func (s *service) updateMasterFiles(ctx context.Context, rootPath string, animeList []domain.Anime) error {
	am := &domain.AnimeMovies{}
	for _, anime := range animeList {
		if anime.Type == "movie" && anime.TmdbID == 0 {
			am.Add(anime.MainTitle, 0, anime.MalID)
		}
	}

	if err := s.mappingRepo.StoreTMDBMaster(ctx, filepath.Join(rootPath, "tmdb-mal-unmapped.yaml"), am); err != nil {
		return errors.Wrap(err, "failed to store unmapped movies")
	}

	// Update master
	existingMaster, err := s.mappingRepo.GetTMDBMaster(ctx, filepath.Join(rootPath, "tmdb-mal-master.yaml"))
	if err != nil {
		// File doesn't exist yet, create new one
		existingMaster = &domain.AnimeMovies{}
	}

	if err := s.updateMaster(ctx, existingMaster, am, filepath.Join(rootPath, "tmdb-mal-master.yaml")); err != nil {
		return errors.Wrap(err, "failed to update master")
	}

	return nil
}

func (s *service) searchTMDB(ctx context.Context, url string) (*TMDBAPIResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response")
	}

	tmdb := &TMDBAPIResponse{}
	err = json.Unmarshal(body, tmdb)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}

	return tmdb, nil
}

// scoredResult represents a TMDB result with a match score
type scoredResult struct {
	ID    int
	Score float64
}

// findBestMatch finds the best matching TMDB result using a scoring system
func (s *service) findBestMatch(anime domain.Anime, results []struct {
	Adult            bool    `json:"adult"`
	BackdropPath     string  `json:"backdrop_path"`
	GenreIds         []int   `json:"genre_ids"`
	ID               int     `json:"id"`
	OriginalLanguage string  `json:"original_language"`
	OriginalTitle    string  `json:"original_title"`
	Overview         string  `json:"overview"`
	Popularity       float64 `json:"popularity"`
	PosterPath       string  `json:"poster_path"`
	ReleaseDate      string  `json:"release_date"`
	Title            string  `json:"title"`
	Video            bool    `json:"video"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
}) *scoredResult {
	if len(results) == 0 {
		return nil
	}

	// If only one result, use it (but still score it)
	if len(results) == 1 {
		score := s.calculateScore(anime, &results[0])
		if score > 0 {
			return &scoredResult{Score: score, ID: results[0].ID}
		}
		return nil
	}

	var bestMatch *scoredResult
	malYear := s.getYear(anime.ReleaseDate)

	for i := range results {
		result := &results[i]

		// Skip videos (trailers, behind-the-scenes, etc.)
		if result.Video {
			continue
		}

		// Skip documentaries (genre 99)
		isDocumentary := false
		for _, genreID := range result.GenreIds {
			if genreID == 99 {
				isDocumentary = true
				break
			}
		}
		if isDocumentary {
			continue
		}

		// Must be same year
		tmdbYear := s.getYear(result.ReleaseDate)
		if tmdbYear != malYear {
			continue
		}

		score := s.calculateScore(anime, result)
		if score > 0 && (bestMatch == nil || score > bestMatch.Score) {
			bestMatch = &scoredResult{Score: score, ID: result.ID}
		}
	}

	return bestMatch
}

// calculateScore calculates a match score for a TMDB result
func (s *service) calculateScore(anime domain.Anime, result *struct {
	Adult            bool    `json:"adult"`
	BackdropPath     string  `json:"backdrop_path"`
	GenreIds         []int   `json:"genre_ids"`
	ID               int     `json:"id"`
	OriginalLanguage string  `json:"original_language"`
	OriginalTitle    string  `json:"original_title"`
	Overview         string  `json:"overview"`
	Popularity       float64 `json:"popularity"`
	PosterPath       string  `json:"poster_path"`
	ReleaseDate      string  `json:"release_date"`
	Title            string  `json:"title"`
	Video            bool    `json:"video"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
}) float64 {
	score := 0.0

	// Title matching (40 points max)
	malTitle := strings.ToLower(strings.TrimSpace(anime.MainTitle))
	malEnglishTitle := strings.ToLower(strings.TrimSpace(anime.EnglishTitle))
	malJapaneseTitle := strings.ToLower(strings.TrimSpace(anime.JapaneseTitle))
	tmdbTitle := strings.ToLower(strings.TrimSpace(result.Title))
	tmdbOriginalTitle := strings.ToLower(strings.TrimSpace(result.OriginalTitle))

	// Check exact matches first (highest priority)
	if malTitle == tmdbTitle || malEnglishTitle == tmdbTitle {
		score += 40 // Exact match
	} else if malTitle == tmdbOriginalTitle || malEnglishTitle == tmdbOriginalTitle {
		score += 35 // Exact match with original title
	} else if malJapaneseTitle != "" && (malJapaneseTitle == tmdbOriginalTitle || malJapaneseTitle == tmdbTitle) {
		score += 35 // Exact match with Japanese title (important for fallback searches)
	} else if strings.Contains(tmdbTitle, malTitle) || strings.Contains(malTitle, tmdbTitle) {
		score += 25 // Partial match
	} else if strings.Contains(tmdbOriginalTitle, malTitle) || strings.Contains(malTitle, tmdbOriginalTitle) {
		score += 20 // Partial match with original title
	} else if malEnglishTitle != "" && (strings.Contains(tmdbTitle, malEnglishTitle) || strings.Contains(malEnglishTitle, tmdbTitle)) {
		score += 25 // Partial match with English title
	} else if malJapaneseTitle != "" && (strings.Contains(tmdbOriginalTitle, malJapaneseTitle) || strings.Contains(malJapaneseTitle, tmdbOriginalTitle)) {
		score += 20 // Partial match with Japanese title
	} else {
		// No title match - very unlikely to be correct
		return 0
	}

	// Date matching (30 points max)
	if result.ReleaseDate == anime.ReleaseDate {
		score += 30 // Exact date match
	} else {
		// Same year, calculate days difference
		malDate, err1 := time.Parse("2006-01-02", anime.ReleaseDate)
		tmdbDate, err2 := time.Parse("2006-01-02", result.ReleaseDate)
		if err1 == nil && err2 == nil {
			daysDiff := int(math.Abs(malDate.Sub(tmdbDate).Hours() / 24))
			if daysDiff <= 7 {
				score += 25 // Within 1 week
			} else if daysDiff <= 30 {
				score += 20 // Within 1 month
			} else if daysDiff <= 90 {
				score += 15 // Within 3 months
			} else {
				score += 10 // Same year but > 3 months
			}
		} else {
			// Can't parse dates, but same year (already checked)
			score += 10
		}
	}

	// Popularity and vote count (20 points max)
	// Normalize popularity (typical range 0-100, but can be higher)
	popularityScore := math.Min(result.Popularity/10.0, 10.0) // Max 10 points
	score += popularityScore

	// Vote count (more votes = more reliable)
	voteScore := math.Min(float64(result.VoteCount)/500.0, 10.0) // Max 10 points (5000+ votes = 10)
	score += voteScore

	// Genre bonus: Animation (16) is a good sign for anime movies (5 points)
	for _, genreID := range result.GenreIds {
		if genreID == 16 { // Animation
			score += 5
			break
		}
	}

	return score
}

// tryFallbackSearches attempts to find a match using synonyms and Japanese titles from MAL
func (s *service) tryFallbackSearches(ctx context.Context, anime domain.Anime, baseURL *url.URL, year string) *scoredResult {
	var bestMatch *scoredResult
	bestScore := 0.0

	// Try Japanese title (already stored in anime struct)
	if anime.JapaneseTitle != "" && anime.JapaneseTitle != anime.MainTitle {
		match := s.searchWithTitle(ctx, anime, baseURL, year, anime.JapaneseTitle, "Japanese title")
		if match != nil && match.Score > bestScore {
			bestMatch = match
			bestScore = match.Score
		}
	}

	// Try synonyms (already stored in anime struct)
	for _, synonym := range anime.Synonyms {
		if synonym != "" && synonym != anime.MainTitle && synonym != anime.EnglishTitle {
			match := s.searchWithTitle(ctx, anime, baseURL, year, synonym, "synonym")
			if match != nil && match.Score > bestScore {
				bestMatch = match
				bestScore = match.Score
			}
		}
	}

	// Try title variations (normalize common patterns)
	variations := s.generateTitleVariations(anime.MainTitle)
	if anime.EnglishTitle != "" {
		variations = append(variations, s.generateTitleVariations(anime.EnglishTitle)...)
	}

	for _, variation := range variations {
		if variation != "" && variation != anime.MainTitle && variation != anime.EnglishTitle {
			match := s.searchWithTitle(ctx, anime, baseURL, year, variation, "title variation")
			if match != nil && match.Score > bestScore {
				bestMatch = match
				bestScore = match.Score
			}
		}
	}

	return bestMatch
}

// searchWithTitle performs a TMDB search with a specific title and returns the best match
func (s *service) searchWithTitle(ctx context.Context, anime domain.Anime, baseURL *url.URL, year, searchTitle, searchType string) *scoredResult {
	target := *baseURL
	query := target.Query()
	query.Set("query", searchTitle)
	query.Set("year", year)
	target.RawQuery = query.Encode()

	s.log.Trace().
		Str("title", anime.MainTitle).
		Int("mal_id", anime.MalID).
		Str("search_title", searchTitle).
		Str("search_type", searchType).
		Msg("Trying fallback search")

	tmdb, err := s.searchTMDB(ctx, target.String())
	if err != nil {
		s.log.Debug().Err(err).Str("search_title", searchTitle).Msg("fallback search failed")
		return nil
	}

	if tmdb.TotalResults == 0 {
		return nil
	}

	return s.findBestMatch(anime, tmdb.Results)
}

// generateTitleVariations generates common title variations for fallback searches
func (s *service) generateTitleVariations(title string) []string {
	variations := []string{}
	title = strings.TrimSpace(title)

	// Remove common suffixes
	suffixes := []string{" the Movie", " Movie", " (Movie)", " - Movie"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(strings.ToLower(title), strings.ToLower(suffix)) {
			variations = append(variations, strings.TrimSuffix(title, suffix))
		}
	}

	// Normalize "vs." variations
	if strings.Contains(title, " vs. ") {
		variations = append(variations, strings.ReplaceAll(title, " vs. ", " vs "))
		variations = append(variations, strings.ReplaceAll(title, " vs. ", " versus "))
	}
	if strings.Contains(title, " vs ") {
		variations = append(variations, strings.ReplaceAll(title, " vs ", " vs. "))
		variations = append(variations, strings.ReplaceAll(title, " vs ", " versus "))
	}

	// Remove extra whitespace
	normalized := strings.Join(strings.Fields(title), " ")
	if normalized != title {
		variations = append(variations, normalized)
	}

	return variations
}

func (s *service) buildUrl(apikey string) *url.URL {
	baseUrl := "https://api.themoviedb.org/3/search/movie"
	u, err := url.Parse(baseUrl)
	if err != nil {
		log.Fatal(err)
	}

	query := u.Query()
	query.Add("api_key", apikey)
	query.Add("language", "en-US")
	query.Add("page", "1")
	query.Add("include_adult", "true")
	u.RawQuery = query.Encode()
	return u
}

func (s *service) getYear(d string) string {
	r := regexp.MustCompile(`^\d{4,4}`)
	return r.FindString(d)
}

func (s *service) updateMaster(ctx context.Context, existing, new *domain.AnimeMovies, path string) error {
	malidToTmdbid := map[int]int{}
	if existing != nil {
		for i := range existing.AnimeMovie {
			if existing.AnimeMovie[i].TMDBID != 0 {
				malidToTmdbid[existing.AnimeMovie[i].MALID] = existing.AnimeMovie[i].TMDBID
			}
		}
	}

	for ii := range new.AnimeMovie {
		if tmdbid, found := malidToTmdbid[new.AnimeMovie[ii].MALID]; found {
			new.AnimeMovie[ii].TMDBID = tmdbid
		}
	}

	return s.mappingRepo.StoreTMDBMaster(ctx, path, new)
}
