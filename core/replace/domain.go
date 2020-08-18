package replace

import (
	"bufio"
	"os"
	"strings"
	"time"

	"github.com/shawn1m/overture/core/errors"
	"github.com/shawn1m/overture/core/finder"
	log "github.com/sirupsen/logrus"
)

type DomainReplace struct {
	filePath string
	finder   finder.Finder
}

func NewDomainReplace(path string, finder finder.Finder) (*DomainReplace, error) {
	if path == "" {
		return nil, nil
	}

	r := &DomainReplace{filePath: path, finder: finder}
	if err := r.initReplace(); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *DomainReplace) Find(name string) string {
	name = strings.TrimSuffix(name, ".")
	lines := r.finder.Get(name)
	if len(lines) > 0 {
		return lines[0]
	}
	return ""
}

func (r *DomainReplace) initReplace() error {
	f, err := os.Open(r.filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	defer log.Debugf("%s took %s", "Load domain replace", time.Since(time.Now()))

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if err := r.parseLine(scanner.Text()); err != nil {
			log.Warnf("Bad formatted replace file line: %s", err)
		}
	}
	return nil
}

func (r *DomainReplace) parseLine(line string) error {
	if len(line) == 0 {
		return nil
	}

	// Parse leading # for disabled lines
	if line[0:1] == "#" {
		return nil
	}

	// Parse other #s for actual comments
	line = strings.Split(line, "#")[0]

	// Replace tabs and spaces with single spaces throughout
	line = strings.Replace(line, "\t", " ", -1)
	for strings.Contains(line, "  ") {
		line = strings.Replace(line, "  ", " ", -1)
	}

	line = strings.TrimSpace(line)

	// Break line into words
	words := strings.Split(line, " ")

	if len(words) < 2 {
		log.Warn("Wrong format")
		return &errors.NormalError{Message: "Wrong format"}
	}
	for i, word := range words {
		words[i] = strings.TrimSpace(word)
	}
	// Separate the first bit (the ip) from the other bits (the domains)
	key, value := words[0], words[1]

	err := r.finder.Insert(key, value)
	if err != nil {
		return err
	}
	return nil
}
