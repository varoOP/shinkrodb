package dedupe

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/varoOP/shinkrodb/internal/domain"
)

type Service interface {
	CheckDupes(ctx context.Context, anime []domain.Anime) (int, []domain.Anime, error)
}

type service struct {
	log        zerolog.Logger
	animeRepo  domain.AnimeRepository
	aidTitleMap map[int]string
}

func NewService(log zerolog.Logger, animeRepo domain.AnimeRepository) Service {
	return &service{
		log:         log.With().Str("module", "dedupe").Logger(),
		animeRepo:   animeRepo,
		aidTitleMap: make(map[int]string),
	}
}

func (s *service) CheckDupes(ctx context.Context, anime []domain.Anime) (int, []domain.Anime, error) {
	dupeanidb := []domain.Anime{}
	indexes := []int{}

	// Find duplicates
	for i, v := range anime {
		if v.AnidbID == 0 {
			continue
		}

		count := 0
		for _, vv := range anime {
			if v.AnidbID == vv.AnidbID && v.Type == vv.Type && v.Type == "tv" {
				count++
			}
		}

		if count > 1 {
			dupeanidb = append(dupeanidb, v)
			indexes = append(indexes, i)
			// Initialize map entry if needed
			if _, ok := s.aidTitleMap[v.AnidbID]; !ok {
				s.aidTitleMap[v.AnidbID] = ""
			}
		}
	}

	// Fill title map if needed
	if len(s.aidTitleMap) > 0 {
		if err := s.fillAidTitleMap(ctx); err != nil {
			s.log.Warn().Err(err).Msg("failed to fill AniDB title map")
		}
	}

	sort.SliceStable(dupeanidb, func(i, j int) bool {
		return dupeanidb[i].AnidbID < dupeanidb[j].AnidbID
	})

	if len(indexes) == 0 {
		return 0, anime, nil
	}

	s.log.Info().Int("dupe_count", len(dupeanidb)).Msg("Found duplicates")

	// Check titles and remove non-matching entries
	deduped := s.checkTitle(anime, indexes)

	return len(dupeanidb), deduped, nil
}

func (s *service) checkTitle(anime []domain.Anime, indexes []int) []domain.Anime {
	for _, index := range indexes {
		if index >= len(anime) {
			continue
		}

		mainTitle := s.aidTitleMap[anime[index].AnidbID]
		if mainTitle != "" && !strings.EqualFold(anime[index].MainTitle, mainTitle) {
			s.log.Debug().
				Int("mal_id", anime[index].MalID).
				Str("anime_title", anime[index].MainTitle).
				Str("anidb_title", mainTitle).
				Msg("Deleting non-matching entry")
			return s.removeIndex(anime, index)
		}
	}
	return anime
}

func (s *service) removeIndex(anime []domain.Anime, index int) []domain.Anime {
	if index < 0 || index >= len(anime) {
		return anime
	}
	return append(anime[:index], anime[index+1:]...)
}

func (s *service) fillAidTitleMap(ctx context.Context) error {
	anidb := &Animetitles{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://github.com/Anime-Lists/anime-lists/raw/master/animetitles.xml", nil)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to fetch animetitles.xml")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	xr := xml.NewDecoder(resp.Body)
	err = xr.Decode(anidb)
	if err != nil {
		return errors.Wrap(err, "failed to decode XML")
	}

	// Fill map for all requested AniDB IDs
	for key := range s.aidTitleMap {
		if s.aidTitleMap[key] != "" {
			continue // Already filled
		}

		for _, anime := range anidb.Anime {
			if anime.Aid == strconv.Itoa(key) {
				for _, title := range anime.Title {
					if title.Type == "main" {
						s.aidTitleMap[key] = title.Text
						break
					}
				}
				break
			}
		}
	}

	return nil
}

type Animetitles struct {
	XMLName xml.Name `xml:"animetitles"`
	Anime   []struct {
		Aid   string `xml:"aid,attr"`
		Title []struct {
			Text string `xml:",chardata"`
			Type string `xml:"type,attr"`
			Lang string `xml:"lang,attr"`
		} `xml:"title"`
	} `xml:"anime"`
}

