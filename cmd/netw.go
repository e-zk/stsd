package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Request and return the value of the HTTP 'Date' header from the given
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

	// get 'Date' header
	date := resp.Header["Date"][0]
	if date == "" {
		date = resp.Header["date"][0]
	}

	// if there's no date (this should never happen)
	if date == "" {
		return "", errors.New("could not get date from header")
	}

	return date, nil
}

// Randomly select a pool from the list of pools
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
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}

		urls = strings.Split(text, ",")

		// if torProxy was specified in program arguments, use onion. otherwise use
		// clearnet url
		if torProxy.String() != "" && len(urls) > 1 && urls[1] != "" {
			pools = append(pools, urls[1])
		} else {
			// TODO more descriptive error here
			if urls[0] == "" {
				return "", errors.New("error in pool file: invalid syntax")
			}
			// mandate 'https://' prefix
			if !strings.HasPrefix(urls[0], "https://") {
				return "", errors.New("clearnet url does not start with https://")
			}
			pools = append(pools, urls[0])
		}
		count++
	}

	// return random url from pools array
	return pools[rand.Intn(count)], nil
}

// Network subprocess logic is as follows.
// 1. Select a random pool
// 2. Get date from http header
// 3. Connect to socket, send date to parent
// 4. Die
func NetworkLogic() {
	log.SetPrefix("(network process) ")

	// 1. select a random pool
	pool, err := selectPool()
	if err != nil {
		log.Fatalf("error getting pool: %v", err)
	}
	log.Printf("selected pool url: %s", pool)

	// 2. get date from the selected pool url
	date, err := getDateFrom(pool)
	if err != nil {
		log.Fatalf("error getting time: %v", err)
	}
	log.Printf("got date: %s", date)

	// 3. send date to parent via socket
	c, err := net.Dial("unixpacket", defaultSocket)
	if err != nil {
		log.Fatalf("error attaching to socket: %v", err)
	}
	_, err = c.Write([]byte(date))
	if err != nil {
		log.Fatalf("error writing to socket: %v", err)
	}

	// 4.
	os.Exit(0)
}
