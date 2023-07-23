package config

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/varoOP/shinkrodb/internal/domain"
)

func NewConfig() *domain.Config {
	s, err := os.Open("./secrets.json")
	if err != nil {
		log.Fatal(fmt.Errorf("unable to find secrets.json. error: %v", err))
	}

	d, err := io.ReadAll(s)
	if err != nil {
		log.Fatal(err)
	}

	c := &domain.Config{}
	err = json.Unmarshal(d, c)
	if err != nil {
		log.Fatal(fmt.Errorf("unable to unmarshal secrets.json. error: %v", err))
	}

	return c
}
