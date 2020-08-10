package querylog

import (
	"log"
	"os"
	"strings"
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

func Log(ip string, domain string, tag string) {
	logger.Printf("%s: %s [%s]\n", ip, strings.TrimRight(domain, "."), tag)
}
