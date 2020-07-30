package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	searchWord = "Go"
	limit      = 5
)

func main() {
	quota := make(chan struct{}, limit)
	result := make(chan int)
	wg := &sync.WaitGroup{}

	//to avoid a race with wg.Wait()
	wg.Add(1)

	go func() {
		defer wg.Done()

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			source := scanner.Text()
			quota <- struct{}{}
			wg.Add(1)
			go worker(source, wg, quota, result)
		}

		if err := scanner.Err(); err != nil {
			log.Fatal("Scanner error:", err)
		}
	}()

	quit := make(chan int)
	go func() {
		var total int
		for {
			select {
			case workerResult := <-result:
				total += workerResult
			case <-quit:
				quit <- total
				return
			}
		}
	}()

	wg.Wait()
	quit <- 0
	fmt.Fprintf(os.Stdout, "Total:%d\n", <-quit)
	close(quit)
	close(result)
	close(quota)
}

func worker(source string, wg *sync.WaitGroup, quota chan struct{}, result chan int) {
	defer func() {
		<-quota
		wg.Done()
	}()

	var (
		res int
		err error
	)

	if isURL(source) {
		res, err = getHTTPCount(source)
	} else {
		res, err = getFileCount(source)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error for %s: %v\n", source, err)
		return
	}

	fmt.Fprintf(os.Stdout, "Count for %s: %v\n", source, res)
	result <- res
}

func isURL(source string) bool {
	_, err := url.ParseRequestURI(source)
	if err != nil {
		return false
	}

	url, err := url.Parse(source)
	if err != nil || url.Scheme == "" || url.Host == "" {
		return false
	}

	return true
}

func getHTTPCount(url string) (int, error) {
	client := http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(url)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return 0, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	return bytes.Count(data, []byte(searchWord)), nil
}

func getFileCount(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineCount := strings.Count(scanner.Text(), searchWord)
		count += lineCount
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return count, nil
}
