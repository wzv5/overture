package querylog

import (
	"log"
	"os"
	"strings"

	"github.com/miekg/dns"
)

var logger *log.Logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)

func SetQueryLogFile(filename string) error {
	logfile, err := os.Create(filename)
	if err != nil {
		return err
	}
	logger.SetOutput(logfile)
	return nil
}

func Log(ip string, domain string, query uint16, tag string) {
	logger.Printf("%s: %s %s [%s]\n", ip, strings.TrimRight(domain, "."), dns.Type(query).String(), tag)
}
