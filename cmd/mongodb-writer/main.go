//
// Copyright (c) 2018
// Mainflux
//
// SPDX-License-Identifier: Apache-2.0
//

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	"github.com/mainflux/mainflux"
	"github.com/mainflux/mainflux/logger"
	"github.com/mainflux/mainflux/writers"
	"github.com/mainflux/mainflux/writers/mongodb"
	nats "github.com/nats-io/go-nats"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	queue = "mongodb-writer"

	defNatsURL  = nats.DefaultURL
	defLogLevel = "error"
	defPort     = "8180"
	defDBName   = "mainflux"
	defDBHost   = "localhost"
	defDBPort   = "27017"

	envNatsURL  = "MF_NATS_URL"
	envLogLevel = "MF_MONGO_WRITER_LOG_LEVEL"
	envPort     = "MF_MONGO_WRITER_PORT"
	envDBName   = "MF_MONGO_WRITER_DB_NAME"
	envDBHost   = "MF_MONGO_WRITER_DB_HOST"
	envDBPort   = "MF_MONGO_WRITER_DB_PORT"
)

type config struct {
	NatsURL  string
	LogLevel string
	Port     string
	DBName   string
	DBHost   string
	DBPort   string
}

func main() {
	cfg := loadConfigs()
	logger, err := logger.New(os.Stdout, cfg.LogLevel)
	if err != nil {
		log.Fatalf(err.Error())
	}
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to connect to NATS: %s", err))
		os.Exit(1)
	}
	defer nc.Close()

	addr := fmt.Sprintf("mongodb://%s:%s", cfg.DBHost, cfg.DBPort)
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(addr))
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to connect to database: %s", err))
		os.Exit(1)
	}

	db := client.Database(cfg.DBName)
	repo := mongodb.New(db)

	counter, latency := makeMetrics()
	repo = writers.LoggingMiddleware(repo, logger)
	repo = writers.MetricsMiddleware(repo, counter, latency)
	if err := writers.Start(nc, repo, queue, logger); err != nil {
		logger.Error(fmt.Sprintf("Failed to start message writer: %s", err))
		os.Exit(1)
	}

	errs := make(chan error, 2)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT)
		errs <- fmt.Errorf("%s", <-c)
	}()

	go startHTTPService(cfg.Port, logger, errs)

	err = <-errs
	logger.Error(fmt.Sprintf("MongoDB writer service terminated: %s", err))
}

func loadConfigs() config {
	return config{
		NatsURL:  mainflux.Env(envNatsURL, defNatsURL),
		LogLevel: mainflux.Env(envLogLevel, defLogLevel),
		Port:     mainflux.Env(envPort, defPort),
		DBName:   mainflux.Env(envDBName, defDBName),
		DBHost:   mainflux.Env(envDBHost, defDBHost),
		DBPort:   mainflux.Env(envDBPort, defDBPort),
	}
}

func makeMetrics() (*kitprometheus.Counter, *kitprometheus.Summary) {
	counter := kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Namespace: "mongodb",
		Subsystem: "message_writer",
		Name:      "request_count",
		Help:      "Number of database inserts.",
	}, []string{"method"})

	latency := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "mongodb",
		Subsystem: "message_writer",
		Name:      "request_latency_microseconds",
		Help:      "Total duration of inserts in microseconds.",
	}, []string{"method"})

	return counter, latency
}

func startHTTPService(port string, logger logger.Logger, errs chan error) {
	p := fmt.Sprintf(":%s", port)
	logger.Info(fmt.Sprintf("Mongodb writer service started, exposed port %s", p))
	errs <- http.ListenAndServe(p, mongodb.MakeHandler())
}
