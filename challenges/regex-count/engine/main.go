package main

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"regexp"
	"time"
)

func main() {
	sleep := flag.Float64("sleep", 0, "fractional seconds to sleep before evaluating (simulates a slow engine)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: engine [--sleep=N] <regex> <seed-hex>")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		flag.Usage()
		os.Exit(2)
	}

	if *sleep > 0 {
		time.Sleep(time.Duration(*sleep * float64(time.Second)))
	}

	re, err := regexp.Compile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid regex:", err)
		os.Exit(1)
	}

	seed, err := hex.DecodeString(args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid seed hex:", err)
		os.Exit(1)
	}

	digest := sha512.Sum512(seed)
	encoded := base64.StdEncoding.EncodeToString(digest[:])

	matches := re.FindAllString(encoded, -1)
	fmt.Println(len(matches))
}
