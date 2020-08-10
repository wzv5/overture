package replace

import (
	"bufio"
	"net"
	"os"
	"strings"
	"time"

	"github.com/shawn1m/overture/core/common"
	"github.com/shawn1m/overture/core/errors"
	"github.com/shawn1m/overture/core/finder"
	log "github.com/sirupsen/logrus"
)

type IPReplace struct {
	filePath string
	lines    []*ipLine
}

type ipLine struct {
	ipset *common.IPSet
	ip    net.IP
}

func NewIPReplace(path string, finder finder.Finder) (*IPReplace, error) {
	if path == "" {
		return nil, nil
	}

	r := &IPReplace{filePath: path, lines: make([]*ipLine, 0)}
	if err := r.initReplace(); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *IPReplace) Find(ip net.IP) net.IP {
	for _, i := range r.lines {
		if i.ipset.Contains(ip, false, "IPReplace") {
			return i.ip
		}
	}
	return nil
}

func (r *IPReplace) initReplace() error {
	f, err := os.Open(r.filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	defer log.Debugf("%s took %s", "Load IP replace", time.Since(time.Now()))

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if err := r.parseLine(scanner.Text()); err != nil {
			log.Warnf("Bad formatted replace file line: %s", err)
		}
	}
	return nil
}

func (r *IPReplace) parseLine(line string) error {
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
	w0, w1 := words[0], words[1]
	_, ipnet, err := net.ParseCIDR(w0)
	if err != nil {
		return err
	}
	ip := net.ParseIP(w1)
	if ip == nil {
		log.Warn("Wrong IP format")
		return &errors.NormalError{Message: "Wrong IP format"}
	}
	ipline := &ipLine{
		ipset: common.NewIPSet([]*net.IPNet{ipnet}),
		ip:    ip,
	}
	r.lines = append(r.lines, ipline)
	return nil
}
