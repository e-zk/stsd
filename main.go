package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"time"
)

/**
 * @brief Get date header from given url.
 *
 * Extracts and returns HTTP 'Date' header, enforcing a minimum TLS
 * version of TLS1.2.
 * @param url Url to get date from.
 * @return Date as a string.
 * @return Error returned by http.Client.Get() if any
 */
func getDateFrom(url string) (string, error) {
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

	// make request
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}

	// get 'Date' header
	date := resp.Header["Date"][0]
	if date == "" {
		date = resp.Header["date"][0]
	}

	return date, err
}

const (
	min_sleep = 60
	max_sleep = 180
)

func randomSleep() {
	sleepTime := time.Duration(min_sleep + rand.Int63n(max_sleep-min_sleep+1))
	log.Printf("sleeping for %v\n", sleepTime*time.Minute)
	time.Sleep(sleepTime * time.Minute)
}

// randomly select a pool from the list of pools
func selectPool() string {

}

func main() {
	rand.Seed(time.Now().Unix())

	for {
		date, _ := getDateFrom("https://zakaria.org/")
		fmt.Printf("date -s %s\n", date)
		randomSleep()
	}
}
