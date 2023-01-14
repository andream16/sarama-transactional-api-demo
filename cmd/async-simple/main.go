package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/andream16/sarama-transactional-api-demo/internal/kafka"
	"github.com/andream16/sarama-transactional-api-demo/internal/metric"
	transporthttp "github.com/andream16/sarama-transactional-api-demo/internal/transport/http"

	"github.com/Shopify/sarama"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	metricsHandler, err := metric.NewMetricsHandler()
	if err != nil {
		log.Fatalf("could not create metrics handler: %v", err)
	}

	var (
		metricsSrv = transporthttp.NewServer(":8082", metricsHandler)
		appName    = os.Getenv("TRANSACTION_ID")
		config     = sarama.NewConfig()
	)

	config.ClientID = appName
	config.Producer.Idempotent = true
	config.Producer.Return.Errors = true
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Partitioner = sarama.NewRoundRobinPartitioner
	config.Producer.Transaction.Retry.Backoff = 10
	config.Producer.Transaction.ID = appName
	config.Net.MaxOpenRequests = 1

	client := kafka.NewClient(config)

	defer client.Close()

	producer, err := sarama.NewAsyncProducerFromClient(client)
	if err != nil {
		log.Fatalf("could not create sarama producer: %v", err)
	}

	defer producer.Close()

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
		defer shutdownCancel()

		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("could not successfully shutdown metrics server: %v", err)
		}

		cancel()
		return nil
	})

	g.Go(func() error {
		return metricsSrv.ListenAndServe()
	})

	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case _, ok := <-producer.Successes():
				if !ok {
					metric.IncMessagesSent(appName, 0, false, false, true)
					return nil
				}
				metric.IncMessagesSent(appName, 1, true, true, false)
			}
		}
	})

	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case err, ok := <-producer.Errors():
				if !ok {
					return nil
				}
				log.Printf("could not send message: %v", err.Err)
			}
		}
	})

	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
				if err := producer.BeginTxn(); err != nil {
					log.Printf("could not begin transaction: %v", err)
					continue
				}

				producer.Input() <- &sarama.ProducerMessage{
					Topic: "txs",
					Value: sarama.StringEncoder(appName),
				}

				producer.Input() <- &sarama.ProducerMessage{
					Topic: "txs",
					Value: sarama.StringEncoder(appName),
				}

				if err := producer.CommitTxn(); err != nil {
					log.Printf("could not commit transaction: %v\n", err)
					if err := producer.AbortTxn(); err != nil {
						log.Printf("could not abort transaction: %v", err)
						continue
					}
				}
			}
		}
	})

	_ = g.Wait()
}
