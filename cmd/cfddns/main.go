package main

import (
	"cfddns/cfddns"
	"cfddns/config"
	"cfddns/log"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/pelletier/go-toml/v2"
	flag "github.com/spf13/pflag"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

var (
	configPath = flag.StringP("config", "c", "config.toml", "path to config file")
	debug      = flag.Bool("debug", false, "enable debug output")
	help       = flag.BoolP("help", "h", false, "Print help message")
)

var buildDate string

var conf config.Config

func init() {
	flag.Parse()
	if *help {
		fmt.Println(flag.CommandLine.FlagUsages())
		os.Exit(0)
	}
}

func getInitLogger() context.Context {
	var err error
	var logger *zap.Logger

	if *debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}

	if err != nil {
		fmt.Printf("Failed creating logger: %e\n", err)
		os.Exit(1)
	}

	return log.WithLogger(context.Background(), logger)
}

func getLogger(ctx context.Context) context.Context {
	var logOption zap.Config
	if *debug {
		logOption = zap.NewDevelopmentConfig()
	} else {
		logOption = zap.NewProductionConfig()
	}

	if conf.Log.Level != nil {
		logOption.Level.SetLevel(*conf.Log.Level)
	}

	if conf.Log.Encoding != nil {
		logOption.Encoding = *conf.Log.Encoding
	}

	if conf.Log.InfoPath != nil {
		logOption.OutputPaths = *conf.Log.InfoPath
	}

	if conf.Log.ErrorPath != nil {
		logOption.ErrorOutputPaths = *conf.Log.ErrorPath
	}

	logOption.InitialFields = map[string]interface{}{
		"node": conf.Service.Name,
	}

	logger, err := logOption.Build()
	if err != nil {
		log.S(ctx).Fatalw("cannot build real logger", zap.Error(err))
	}

	return log.WithLogger(context.Background(), logger)
}

func main() {
	ctx := getInitLogger()

	if buildDate != "" {
		log.S(ctx).Infow("cfddns starting", "variant", "release", "build_date", buildDate)
	} else {
		log.S(ctx).Infow("cfddns starting", "variant", "debug")
	}

	f, err := os.Open(*configPath)
	if err != nil {
		log.S(ctx).Fatalw("failed loading config", zap.Error(err))
	}

	switch {
	case strings.HasSuffix(*configPath, ".toml"):
		err = toml.NewDecoder(f).Decode(&conf)
	case strings.HasSuffix(*configPath, ".yaml") || strings.HasSuffix(*configPath, ".yml"):
		err = yaml.NewDecoder(f).Decode(&conf)
	case strings.HasSuffix(*configPath, ".json"):
		err = json.NewDecoder(f).Decode(&conf)
	}

	if err != nil {
		log.S(ctx).Fatalw("failed loading config", zap.Error(err))
	}

	if conf.Service.Name != "" {
		cfddns.DefaultMark += "-" + conf.Service.Name
	}

	ctx = getLogger(ctx)

	resolver, err := cfddns.NewResolver(ctx, conf.Address)
	if err != nil {
		log.S(ctx).Fatalw("cannot init resolver", zap.Error(err))
	}

	publisher, err := cfddns.NewPublisher(ctx, conf.Provider, conf.Domain)
	if err != nil {
		log.S(ctx).Fatalw("cannot init publisher", zap.Error(err))
	}

	var ticker *time.Ticker
	if conf.Service.RefreshRate > 0 {
		ticker = time.NewTicker(time.Duration(conf.Service.RefreshRate))
	}

	for {
		result, err := resolver.Resolve(ctx)
		if err != nil {
			log.S(ctx).Errorw("resolve failed, skip update", zap.Error(err))
			goto EndUpdate
		}

		err = publisher.Publish(ctx, result)
		if err != nil {
			log.S(ctx).Errorw("publish failed", zap.Error(err))
		}

	EndUpdate:
		if ticker == nil {
			os.Exit(0)
		}

		<-ticker.C
	}
}
