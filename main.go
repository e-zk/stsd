package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	defaultTorProxy = "socks5://127.0.0.1:9050"
	defaultPoolFile = "/etc/stsd_pool"
	minSleep        = 60
	maxSleep        = 180
	usageText       = `secure time sync daemon
usage: stsd [-b] [--pool-file=file] [--use-proxy=proxy | --use-tor[=proxy]]
where:
  -b                 force using bsd date command syntax when setting date.
  --pool-file=file   use the specified pool file (default: /etc/stsd_pool).
  --use-proxy=proxy  proxy network requests through 'proxy' url.
  --use-tor          use tor for network requests. favours onion addresses
                     from the pool file. tor's proxy url can be configured
                     by passing as an argument flag: '--use-tor=proxy'
                     (default tor proxy url: 'socks5://localhost:9050').
`
)

var (
	torProxy torFlag
	useProxy string
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

// requests and returns the value of the HTTP 'Date' header from the given
// url. enforces a minimum TLS version of TLS1.2 to prevent SSL downgrade
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

	// use tor socks proxy, or user proxy if specified
	if torProxy.String() != "" {
		proxyUrl, err := url.Parse(torProxy.proxyUrl)
		if err != nil {
			return "", fmt.Errorf("could not parse proxy url '%s': %v", proxyUrl, err)
		}

		log.Printf("using tor proxy: '%s'", proxyUrl.String())

		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
			DialTLS: func(network, addr string) (net.Conn, error) {
				conn, err := tls.Dial(network, addr, tlsConf)
				return conn, err
			},
		}
	} else if useProxy != "" {
		proxyUrl, err := url.Parse(useProxy)
		if err != nil {
			return "", fmt.Errorf("could not parse proxy url '%s': %v", proxyUrl, err)
		}

		log.Printf("using proxy: '%s'", proxyUrl.String())

		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
			DialTLS: func(network, addr string) (net.Conn, error) {
				conn, err := tls.Dial(network, addr, tlsConf)
				return conn, err
			},
		}
	}

	// make request
	resp, err := client.Get(pool)
	if err != nil {
		return "", err
	}

	// TODO remove debug printing of headers
	for k, v := range resp.Header {
		log.Printf(">> %v : %v\n", k, v)
	}

	// get 'Date' header
	date := resp.Header["Date"][0]
	if date == "" {
		date = resp.Header["date"][0]
	}

	// if there's no date
	if date == "" {
		return "", errors.New("could not get date from header")
	}

	return date, nil
}

// sleep for random amount of time
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
		urls = strings.Split(text, ",")

		// if torProxy is specified, use onion. otherwise use clearnet url
		if torProxy.String() != "" && len(urls) > 1 && urls[1] != "" {
			pools = append(pools, urls[1])
		} else {
			pools = append(pools, urls[0])
		}
		count++
	}

	// return random url from pools array
	return pools[rand.Intn(count)], nil
}

// update system date
func updateDate() {
	// select a random pool
	pool, err := selectPool()
	if err != nil {
		log.Fatalf("error getting pool: %v", err)
	}

	log.Printf("selected pool url: %s", pool)
	// get date from the selected pool url
	date, err := getDateFrom(pool)
	if err != nil {
		log.Fatalf("error getting time: %v", err)
	}

	log.Printf("got date: %s", date)

	// TODO actually set system date
	fmt.Printf(">> date -s %s\n", date)
}

func main() {
	rand.Seed(time.Now().Unix())

	// setup flags
	flag.BoolVar(&bsdDate, "b", false, "use bsd date command format")
	flag.StringVar(&useProxy, "use-proxy", "", "use specified proxy")
	flag.Var(&torProxy, "use-tor", "use tor")
	flag.StringVar(&poolFile, "pool-file", defaultPoolFile, "pool file to use")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usageText)
	}
	flag.Parse()

	// if passed w/o an argument set it to the default proxy url
	if torProxy.String() == "true" {
		torProxy.Set(defaultTorProxy)
	}

	// if both --use-proxy and --use-tor are set; throw an error
	if torProxy.String() != "" && useProxy != "" {
		log.Fatalf("error: cannot use --use-tor option with --use-proxy.")
	}

	// setup sigusr1 handler
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGUSR1)
	go func() {
		for {
			sig := <-sc
			switch sig {
			case syscall.SIGUSR1:
				log.Printf("received SIGUSR1. forcing time update.")
				updateDate()
			}
		}
	}()

	for {
		updateDate()
		randomSleep()
	}
}
