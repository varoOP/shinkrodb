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
	ScrapeAniDBIDs(ctx context.Context, cacheDir string) error
}

type service struct {
	log        zerolog.Logger
	config     *domain.Config
	animeRepo  domain.AnimeRepository
	malIDPath  domain.AnimePath
	anidbPath  domain.AnimePath
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

	// Copy to anidb path if it doesn't exist
	if err := s.copyFileIfNotExist(ctx, s.malIDPath, s.anidbPath); err != nil {
		return errors.Wrap(err, "failed to copy file")
	}

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

func (s *service) ScrapeAniDBIDs(ctx context.Context, cacheDir string) error {
	cc := colly.NewCollector(
		colly.AllowedDomains("myanimelist.net"),
		colly.CacheDir(cacheDir),
	)

	extensions.RandomUserAgent(cc)

	a, err := s.animeRepo.Get(ctx, s.anidbPath)
	if err != nil {
		return errors.Wrap(err, "failed to get anime list")
	}

	// Update from existing AniDB data if available
	a = s.updateMalfromAnidb(a)

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

	cc.Limit(&colly.LimitRule{
		RandomDelay: 5 * time.Second,
		Delay:       5 * time.Second,
		Parallelism: 10,
		DomainGlob:  "*myanimelist*",
	})

	cc.OnRequest(func(r *colly.Request) {
		s.log.Debug().Str("url", r.URL.String()).Msg("visiting")
	})

	for _, v := range a {
		if v.AnidbID <= 0 {
			cc.Visit(fmt.Sprintf("https://myanimelist.net/anime/%d", v.MalID))
		}
	}

	if err := s.animeRepo.Store(ctx, s.anidbPath, a); err != nil {
		return errors.Wrap(err, "failed to store AniDB IDs")
	}

	return nil
}

func (s *service) updateMalfromAnidb(anime []domain.Anime) []domain.Anime {
	// This would load from a pre-existing mapping if available
	// For now, just return the anime list as-is
	return anime
}

func (s *service) copyFileIfNotExist(ctx context.Context, srcPath, dstPath domain.AnimePath) error {
	// Check if destination exists
	if _, err := s.animeRepo.Get(ctx, dstPath); err == nil {
		s.log.Debug().Str("path", string(dstPath)).Msg("File already exists, skipping copy")
		return nil
	}

	// Copy source to destination
	src, err := s.animeRepo.Get(ctx, srcPath)
	if err != nil {
		return errors.Wrap(err, "failed to read source file")
	}

	if err := s.animeRepo.Store(ctx, dstPath, src); err != nil {
		return errors.Wrap(err, "failed to write destination file")
	}

	s.log.Debug().Str("src", string(srcPath)).Str("dst", string(dstPath)).Msg("File copied")
	return nil
}

