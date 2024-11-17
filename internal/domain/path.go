package domain

import "path/filepath"

type AnimeFile string

const malidFile AnimeFile = "malid.json"
const anidbFile AnimeFile = "malid-anidbid.json"
const tvdbFile AnimeFile = "malid-anidbid-tvdbid.json"
const tmdbFile AnimeFile = "malid-anidbid-tvdbid-tmdbid.json"
const shinkroFile AnimeFile = "for-shinkro.json"

type AnimePath string

var MalIDPath AnimePath
var AniDBIDPath AnimePath
var TVDBIDPath AnimePath
var TMDBIDPath AnimePath
var shinkroPath AnimePath

func SetAnimePaths(rootDir string) {
	rootDir = setshinkrodb(rootDir)
	MalIDPath = makeAnimePath(rootDir, malidFile)
	AniDBIDPath = makeAnimePath(rootDir, anidbFile)
	TVDBIDPath = makeAnimePath(rootDir, tvdbFile)
	TMDBIDPath = makeAnimePath(rootDir, tmdbFile)
	shinkroPath = makeAnimePath(rootDir, shinkroFile)
}

func makeAnimePath(rootDir string, af AnimeFile) AnimePath {
	return AnimePath(filepath.Join(rootDir, string(af)))
}

func setshinkrodb(rootDir string) string {
	return filepath.Join(rootDir, "shinkrodb")
}
