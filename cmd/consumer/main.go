package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/andream16/sarama-transactional-api-demo/internal/kafka"
	"github.com/andream16/sarama-transactional-api-demo/internal/metric"
	transporthttp "github.com/andream16/sarama-transactional-api-demo/internal/transport/http"

	"github.com/Shopify/sarama"
	"golang.org/x/sync/errgroup"
)

type consumerHandler struct{}

func (c *consumerHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (c *consumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (c *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-session.Context().Done():
			return nil
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			var (
				sender    = string(message.Value)
				offset    = message.Offset
				partition = message.Partition
			)

			log.Printf("offset: %d, partition: %d, value: %v", offset, partition, sender)
			metric.IncMessagesConsumed(sender, offset, partition)
			session.MarkMessage(message, "")
		}
	}
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	metricsHandler, err := metric.NewMetricsHandler()
	if err != nil {
		log.Fatalf("could not create metrics handler: %v", err)
	}

	var (
		metricsSrv             = transporthttp.NewServer(":8082", metricsHandler)
		appName                = os.Getenv("TRANSACTION_ID")
		shouldReadCommitted, _ = strconv.ParseBool(os.Getenv("READ_ONLY_COMMITTED"))
		config                 = sarama.NewConfig()
	)

	config.ClientID = appName
	config.Version = sarama.V2_5_0_0
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.IsolationLevel = sarama.ReadUncommitted

	if shouldReadCommitted {
		config.Consumer.IsolationLevel = sarama.ReadCommitted
	}

	client := kafka.NewClient(config)

	defer client.Close()

	consumer, err := sarama.NewConsumerGroupFromClient(appName, client)
	if err != nil {
		log.Fatalf("could not create consumer: %v", err)
	}

	defer consumer.Close()

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
			if err := consumer.Consume(ctx, []string{"txs"}, &consumerHandler{}); err != nil {
				log.Printf("unexpected consumer error: %v\n", err)
			}
		}
	})

	_ = g.Wait()
}
