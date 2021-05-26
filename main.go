package main

import (
	"context"
	"flag"
	"log"

	"github.com/whywriteit/wwi-agent/cf"
	"golang.org/x/sync/errgroup"
)

var (
	flagCFToken    string
	flagHomeDomain string
)

func init() {
	flag.StringVar(&flagCFToken, "cf-token", "", "token for cloudflare")
	flag.StringVar(&flagHomeDomain, "home-domain", "", "zone for compute")
	flag.Parse()
}

func main() {
	eg, ctx := errgroup.WithContext(context.Background())

	eg.Go(func() error {
		return cf.Loop(ctx, flagCFToken, flagHomeDomain)
	})

	if err := eg.Wait(); err != nil {
		log.Println(err)
	}
}
