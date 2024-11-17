package tvdbmap

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/varoOP/shinkrodb/internal/domain"
	"gopkg.in/yaml.v3"
)

type AnimeTVDBMap struct {
	Anime []Anime `yaml:"AnimeMap"`
}

type Anime struct {
	Malid        int            `yaml:"malid"`
	Title        string         `yaml:"title"`
	Type         string         `yaml:"type"`
	Tvdbid       int            `yaml:"tvdbid"`
	TvdbSeason   int            `yaml:"tvdbseason"`
	Start        int            `yaml:"start"`
	UseMapping   bool           `yaml:"useMapping"`
	AnimeMapping []AnimeMapping `yaml:"animeMapping"`
}

type AnimeMapping struct {
	TvdbSeason int `yaml:"tvdbseason"`
	Start      int `yaml:"start"`
}

func (am *AnimeTVDBMap) Store(path string) error {
	b, err := yaml.Marshal(am)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}

	text := string(b)
	lines := strings.Split(text, "\n")
	malidFound := false
	for i, line := range lines {
		if strings.Contains(line, "malid") {
			if malidFound {
				lines[i-1] += "\n"
			} else {
				malidFound = true
			}
		}
	}

	modifiedText := strings.Join(lines, "\n")
	defer f.Close()
	_, err = f.Write([]byte(modifiedText))
	if err != nil {
		return err
	}

	return nil
}

func GetAnimeTVDBMap(path string) (*AnimeTVDBMap, error) {
	am := &AnimeTVDBMap{}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(b, am)
	if err != nil {
		return nil, err
	}

	return am, nil
}

func CreateAnimeTVDBMap(path string) *AnimeTVDBMap {
	am := &AnimeTVDBMap{}
	a := domain.GetAnime(domain.MalIDPath)
	for _, anime := range a {
		am.Anime = append(am.Anime, Anime{
			anime.MalID,
			anime.MainTitle,
			anime.Type,
			0,
			0,
			0,
			false,
			[]AnimeMapping{},
		})
	}

	am.Store(filepath.Join(path, "tvdb-mal-unmapped.yaml"))
	return am
}

func UpdateMaster(unmapped *AnimeTVDBMap, path string) error {
	master, err := GetAnimeTVDBMap(filepath.Join(path, "tvdb-mal-master.yaml"))
	if err != nil {
		return err
	}

	masterMap := make(map[int]Anime)
	for _, v := range master.Anime {
		if v.Tvdbid != 0 {
			masterMap[v.Malid] = v
		}
	}

	for i, v := range unmapped.Anime {
		if masterAnime, ok := masterMap[v.Malid]; ok {
			unmapped.Anime[i].AnimeMapping = masterAnime.AnimeMapping
			unmapped.Anime[i].Start = masterAnime.Start
			unmapped.Anime[i].TvdbSeason = masterAnime.TvdbSeason
			unmapped.Anime[i].Tvdbid = masterAnime.Tvdbid
			unmapped.Anime[i].UseMapping = masterAnime.UseMapping
		}
	}

	err = unmapped.Store(filepath.Join(path, "tvdb-mal-master.yaml"))
	if err != nil {
		return err
	}

	return nil
}

func GenerateAnimeTVDBMap(path string) error {
	master, err := GetAnimeTVDBMap(filepath.Join(path, "tvdb-mal-master.yaml"))
	if err != nil {
		return err
	}

	final := &AnimeTVDBMap{}
	for _, anime := range master.Anime {
		if anime.Tvdbid != 0 {
			final.Anime = append(final.Anime, anime)
		}
	}

	final.Store(filepath.Join(path, "tvdb-mal.yaml"))
	return nil
}
