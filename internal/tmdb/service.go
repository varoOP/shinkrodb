package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/varoOP/shinkrodb/internal/domain"
)

type Service interface {
	GetTmdbIds(ctx context.Context, rootPath string) error
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
		config:     config,
		animeRepo:  animeRepo,
		mappingRepo: mappingRepo,
		paths:      paths,
	}
}

func (s *service) GetTmdbIds(ctx context.Context, rootPath string) error {
	a, err := s.animeRepo.Get(ctx, s.paths.TVDBPath)
	if err != nil {
		return errors.Wrap(err, "failed to get anime list")
	}

	u := s.buildUrl(s.config.TmdbApiKey)
	am := &domain.AnimeMovies{}
	noTmdbTotal := 0
	withTmdbTotal := 0
	totalMovies := 0

	for i, anime := range a {
		if anime.Type == "movie" {
			totalMovies++
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
				continue
			}

			year := s.getYear(anime.ReleaseDate)
			query.Add("year", year)
			target.RawQuery = query.Encode()

			tmdb, err := s.searchTMDB(ctx, target.String())
			if err != nil {
				s.log.Warn().Err(err).Str("title", anime.MainTitle).Msg("failed to search TMDB")
				continue
			}

			matched := false
			for _, result := range tmdb.Results {
				if result.ReleaseDate == anime.ReleaseDate || tmdb.TotalResults == 1 {
					a[i].TmdbID = result.ID
					withTmdbTotal++
					matched = true
					s.log.Debug().Str("title", anime.MainTitle).Int("tmdb_id", result.ID).Msg("TMDBID added")
					break
				} else {
					s.log.Warn().
						Str("title", anime.MainTitle).
						Str("tmdb_date", result.ReleaseDate).
						Str("mal_date", anime.ReleaseDate).
						Int("total_results", tmdb.TotalResults).
						Msg("TMDB date does not match MAL date and has multiple results")
				}
			}

			if !matched {
				noTmdbTotal++
				am.Add(anime.MainTitle, 0, anime.MalID)
			}
		}
	}

	if err := s.animeRepo.Store(ctx, s.paths.TMDBPath, a); err != nil {
		return errors.Wrap(err, "failed to store TMDB IDs")
	}

	s.log.Info().
		Int("total_movies", totalMovies).
		Int("with_tmdbid", withTmdbTotal).
		Int("without_tmdbid", noTmdbTotal).
		Msg("TMDB ID mapping complete")

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

