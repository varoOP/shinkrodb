package mal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/varoOP/shinkrodb/internal/domain"
)

type Service interface {
	GetAnimeIDs(ctx context.Context) error
	ScrapeAniDBIDs(ctx context.Context, cacheRepo domain.CacheRepo) error
}

type service struct {
	log       zerolog.Logger
	config    *domain.Config
	animeRepo domain.AnimeRepository
	malIDPath domain.AnimePath
	anidbPath domain.AnimePath
}

type MalResponse struct {
	Data []struct {
		Node struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			MainPicture struct {
				Medium string `json:"medium"`
				Large  string `json:"large"`
			} `json:"main_picture"`
			MediaType         string `json:"media_type"`
			AlternativeTitles struct {
				Synonyms []string `json:"synonyms"`
				English  string   `json:"en"`
				Japanese string   `json:"ja"`
			} `json:"alternative_titles"`
			StartDate string `json:"start_date"`
		} `json:"node"`
		Ranking struct {
			Rank int `json:"rank"`
		} `json:"ranking"`
	} `json:"data"`
	Paging struct {
		Next string `json:"next"`
	} `json:"paging"`
}

type clientIDTransport struct {
	Transport http.RoundTripper
	ClientID  string
}

func (c *clientIDTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if c.Transport == nil {
		c.Transport = http.DefaultTransport
	}
	req.Header.Add("X-MAL-CLIENT-ID", c.ClientID)
	return c.Transport.RoundTrip(req)
}

func NewService(log zerolog.Logger, config *domain.Config, animeRepo domain.AnimeRepository, malIDPath, anidbPath domain.AnimePath) Service {
	return &service{
		log:       log.With().Str("module", "mal").Logger(),
		config:    config,
		animeRepo: animeRepo,
		malIDPath: malIDPath,
		anidbPath: anidbPath,
	}
}

func (s *service) GetAnimeIDs(ctx context.Context) error {
	s.log.Info().Msg("Getting current ids from myanimelist..")
	c := &http.Client{
		Transport: &clientIDTransport{ClientID: s.config.MalClientID},
	}

	a := []domain.Anime{}
	next, err := s.storeAnimeID(ctx, c, "https://api.myanimelist.net/v2/anime/ranking?ranking_type=all&limit=500&fields={media_type,start_date,alternative_titles{en}}", &a)
	if err != nil {
		return errors.Wrap(err, "failed to fetch initial MAL IDs")
	}

	for {
		if next != "" {
			next, err = s.storeAnimeID(ctx, c, next, &a)
			if err != nil {
				return errors.Wrap(err, "failed to fetch MAL IDs")
			}
		} else {
			break
		}
	}

	sort.SliceStable(a, func(i, j int) bool {
		return a[i].MalID < a[j].MalID
	})

	if err := s.animeRepo.Store(ctx, s.malIDPath, a); err != nil {
		return errors.Wrap(err, "failed to store MAL IDs")
	}
	s.log.Info().Str("path", string(s.malIDPath)).Msg("Stored malids")

	return nil
}

func (s *service) storeAnimeID(ctx context.Context, c *http.Client, url string, a *[]domain.Anime) (string, error) {
	mal := &MalResponse{}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to create request")
	}

	resp, err := c.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to fetch")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read response body")
	}

	err = json.Unmarshal(body, mal)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal response")
	}

	for _, v := range mal.Data {
		*a = append(*a, domain.Anime{
			MainTitle:    v.Node.Title,
			EnglishTitle: v.Node.AlternativeTitles.English,
			MalID:        v.Node.ID,
			Type:         v.Node.MediaType,
			ReleaseDate:  v.Node.StartDate,
		})
	}

	return mal.Paging.Next, nil
}

func (s *service) ScrapeAniDBIDs(ctx context.Context, cacheRepo domain.CacheRepo) error {
	// Get anime list from malid.json
	a, err := s.animeRepo.Get(ctx, s.malIDPath)
	if err != nil {
		return errors.Wrap(err, "failed to get anime list")
	}

	// Get all cached entries with AniDB IDs (regardless of release date)
	// Entries with AniDB IDs are always valid
	cachedMalIDs := make(map[int]bool)
	if cacheRepo != nil {
		anidbMap, err := cacheRepo.GetAniDBIDs(ctx)
		if err == nil {
			for malID, anidbID := range anidbMap {
				if anidbID > 0 {
					cachedMalIDs[malID] = true
					// Update anime list with cached AniDB ID
					for i := range a {
						if a[i].MalID == malID {
							a[i].AnidbID = anidbID
							break
						}
					}
				}
			}
		}
	}

	// Filter to only scrape entries that:
	// 1. Don't have an AniDB ID in cache
	// 2. Were released in the last year (based on release_date from anime list)
	currentYear := time.Now().Year()
	oneYearAgoYear := currentYear - 1
	toScrape := []domain.Anime{}
	for _, anime := range a {
		if !cachedMalIDs[anime.MalID] && anime.AnidbID <= 0 {
			// Check if released in the last year
			shouldScrape := false
			if anime.ReleaseDate != "" {
				// Extract year from release_date (format: "YYYY-MM-DD", "YYYY-MM", or "YYYY")
				if len(anime.ReleaseDate) >= 4 {
					releaseYearStr := anime.ReleaseDate[:4]
					if releaseYear, parseErr := strconv.Atoi(releaseYearStr); parseErr == nil {
						shouldScrape = releaseYear >= oneYearAgoYear
					}
				}
			} else {
				// If no release date, don't scrape (too old or unknown)
				shouldScrape = false
			}

			if shouldScrape {
				toScrape = append(toScrape, anime)
			}
		}
	}

	if len(toScrape) == 0 {
		s.log.Info().Msg("All anime already cached, skipping scrape")
		// Still store the updated list with cached AniDB IDs
		if err := s.animeRepo.Store(ctx, s.anidbPath, a); err != nil {
			return errors.Wrap(err, "failed to store AniDB IDs")
		}
		return nil
	}

	s.log.Info().Int("total", len(a)).Int("cached", len(cachedMalIDs)).Int("to_scrape", len(toScrape)).Msg("Starting scrape")

	// Use Colly for scraping (no file cache, using database cache instead)
	cc := colly.NewCollector(
		colly.AllowedDomains("myanimelist.net"),
	)

	extensions.RandomUserAgent(cc)

	r := regexp.MustCompile(`aid=(\d+)`)
	cc.OnHTML("a[href]", func(e *colly.HTMLElement) {
		if e.Attr("data-ga-click-type") == "external-links-anime-pc-anidb" {
			url := e.Attr("href")
			m := r.FindStringSubmatch(url)
			if len(m) < 2 {
				return
			}

			anidbid, err := strconv.Atoi(m[1])
			if err != nil {
				s.log.Warn().Err(err).Str("url", url).Msg("failed to parse AniDB ID")
				return
			}

			// Extract MAL ID from URL to find the right anime
			malIDMatch := regexp.MustCompile(`/anime/(\d+)`).FindStringSubmatch(e.Request.URL.String())
			if len(malIDMatch) >= 2 {
				malID, _ := strconv.Atoi(malIDMatch[1])
				for i := range a {
					if a[i].MalID == malID {
						a[i].AnidbID = anidbid
						s.log.Debug().Int("anidbid", anidbid).Int("malid", malID).Msg("Parsed AniDB ID")
						break
					}
				}
			}
		}
	})

	// Since to_scrape count is expected to be low, use higher parallelism and lower delays
	cc.Limit(&colly.LimitRule{
		RandomDelay: 1 * time.Second,
		Delay:       1 * time.Second,
		Parallelism: 30,
		DomainGlob:  "*myanimelist*",
	})

	cc.OnRequest(func(r *colly.Request) {
		s.log.Debug().Str("url", r.URL.String()).Msg("visiting")
	})

	// Only scrape entries not in cache
	for _, v := range toScrape {
		cc.Visit(fmt.Sprintf("https://myanimelist.net/anime/%d", v.MalID))
	}

	// Wait for scraping to complete
	cc.Wait()

	// Update database with newly scraped AniDB IDs
	if err := s.updateCacheDatabase(ctx, cacheRepo, a); err != nil {
		s.log.Warn().Err(err).Msg("failed to update cache database")
	}

	if err := s.animeRepo.Store(ctx, s.anidbPath, a); err != nil {
		return errors.Wrap(err, "failed to store AniDB IDs")
	}

	return nil
}

func (s *service) updateCacheDatabase(ctx context.Context, cacheRepo domain.CacheRepo, animeList []domain.Anime) error {
	// Check if database exists
	if cacheRepo == nil {
		return nil // No cache repository provided, skip update
	}

	now := time.Now().Format(time.RFC3339)
	updated := 0

	for _, anime := range animeList {
		url := fmt.Sprintf("https://myanimelist.net/anime/%d", anime.MalID)
		hadAniDBID := anime.AnidbID > 0

		// Debug: Log values to catch any bugs
		if anime.MalID == anime.AnidbID && anime.AnidbID > 0 {
			s.log.Warn().
				Int("mal_id", anime.MalID).
				Int("anidb_id", anime.AnidbID).
				Msg("WARNING: mal_id equals anidb_id - this should not happen!")
		}

		// Upsert cache entry using repository
		entry := &domain.CacheEntry{
			MalID:       anime.MalID,
			AnidbID:     anime.AnidbID,
			URL:         url,
			CachedAt:    now,
			LastUsed:    now,
			HadAniDBID:  hadAniDBID,
			ReleaseDate: anime.ReleaseDate,
			Type:        anime.Type,
		}

		if err := cacheRepo.UpsertEntry(ctx, entry); err != nil {
			s.log.Warn().Err(err).Int("mal_id", anime.MalID).Msg("failed to upsert cache entry")
			continue
		}
		updated++
	}

	s.log.Debug().Int("updated_count", updated).Msg("Updated cache database with AniDB IDs")
	return nil
}
