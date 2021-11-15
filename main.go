package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultTorProxy = "socks5://127.0.0.1:9050"
	defaultPoolFile = "/etc/stsd_pool"
	minSleep        = 60
	maxSleep        = 180
)

var (
	torProxy torFlag
	bsdDate  bool
	poolFile string
)

// custom flag
type torFlag struct {
	proxyUrl string
}

func (t *torFlag) Set(x string) error {
	t.proxyUrl = x
	return nil
}
func (t *torFlag) String() string {
	return t.proxyUrl
}
func (t *torFlag) IsBoolFlag() bool {
	return true
}

// Requests and returns the value of the HTTP 'Date' header from the given
// url. Enforces a minimum TLS version of TLS1.2 to prevent SSL downgrade
// attacks.
func getDateFrom(pool string) (string, error) {
	// tls config to enforce tls1.2
	tlsConf := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// setup client to use tlsConf
	client := &http.Client{
		Transport: &http.Transport{
			DialTLS: func(network, addr string) (net.Conn, error) {
				conn, err := tls.Dial(network, addr, tlsConf)
				return conn, err
			},
		},
	}

	if torProxy.String() != "" {
		proxyUrl, err := url.Parse(torProxy.proxyUrl)
		if err != nil {
			log.Fatalf("could not parse proxy url '%s'", proxyUrl)
		}

		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
			DialTLS: func(network, addr string) (net.Conn, error) {
				conn, err := tls.Dial(network, addr, tlsConf)
				return conn, err
			},
		}

		// TODO if .onion, don't enforce TLS
		// this may be not neccessary since onions are usually over http:// anyways
		// and go should pick that up and not enforce tls?
	}

	// make request
	resp, err := client.Get(pool)
	if err != nil {
		return "", err
	}

	//
	for k, v := range resp.Header {
		log.Printf(">> %v : %v\n", k, v)
	}

	// get 'Date' header
	date := resp.Header["Date"][0]
	if date == "" {
		date = resp.Header["date"][0]
	}

	return date, err
}

/**
 * Sleep for random amount of time
 */
func randomSleep() {
	sleepTime := time.Duration(minSleep + rand.Int63n(maxSleep-minSleep+1))
	log.Printf("sleeping for %v\n", sleepTime*time.Minute)
	time.Sleep(sleepTime * time.Minute)
}

// randomly select a pool from the list of pools
func selectPool() (string, error) {
	f, err := os.Open(poolFile)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var count int
	var pools []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var (
			text string
			urls []string
		)

		text = scanner.Text()
		urls = strings.Split(text, "|")

		if torProxy.String() != "" && urls[1] != "" {
			pools = append(pools, urls[1])
		} else {
			pools = append(pools, urls[0])
		}
		count++
	}

	return pools[rand.Intn(count)], nil
}

func main() {
	rand.Seed(time.Now().Unix())

	flag.BoolVar(&bsdDate, "b", false, "use bsd date command format")
	flag.Var(&torProxy, "use-tor", "use tor")
	flag.StringVar(&poolFile, "pool-file", defaultPoolFile, "pool file to use")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "secure time sync daemon\n")
		fmt.Fprintf(os.Stderr, "usage: stsd [-b] [--pool-file=file] [--use-tor[=proxy]]\n")
		fmt.Fprintf(os.Stderr, "where:\n")
		fmt.Fprintf(os.Stderr, "  -b                force using bsd date command syntax when setting date\n")
		fmt.Fprintf(os.Stderr, "  --pool-file=file  use the specified pool file\n")
		fmt.Fprintf(os.Stderr, "  --use-tor         use tor for network requests. favours onion addresses from the pool file.\n")
		fmt.Fprintf(os.Stderr, "                    custom proxy URL can be configured by passing as argument: '--use-tor=[url]'.\n")
		fmt.Fprintf(os.Stderr, "                    defaults to 'socks5://localhost:9050'.\n")
	}
	flag.Parse()

	// if passed w/o an argument set it to the default proxy url
	if torProxy.String() == "true" {
		torProxy.Set(defaultTorProxy)
	}

	for {
		log.Printf("proxy: %s\n", torProxy.String())

		// TODO get a random pool
		pool, err := selectPool()
		if err != nil {
			log.Fatalf("error getting pool: %v", err)
		}

		fmt.Printf(">> %s\n", pool)

		// TODO get date from random pool
		date, err := getDateFrom(pool)
		if err != nil {
			log.Fatalf("error getting time: %v", err)
		}

		// TODO set system date
		fmt.Printf(">> date -s %s\n", date)

		randomSleep()
	}
}
