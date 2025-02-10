package kafka

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/IBM/sarama"
)

const (
	maxRetries = 10
	retryDelay = 3 * time.Second
)

func waitForKafka(brokers []string) error {
	for i := 0; i < maxRetries; i++ {
		config := sarama.NewConfig()
		config.Net.DialTimeout = 1 * time.Second
		client, err := sarama.NewClient(brokers, config)
		if err == nil {
			client.Close()
			return nil
		}
		slog.Info("Waiting for Kafka to be ready...", "attempt", i+1)
		time.Sleep(retryDelay)
	}
	return fmt.Errorf("kafka not available after %d attempts", maxRetries)
}

func NewProducer(broker string, retryMax int, retryBackoff int64) (sarama.SyncProducer, error) {
	brokers := []string{broker}
	if err := waitForKafka(brokers); err != nil {
		return nil, err
	}

	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Retry.Max = retryMax
	config.Producer.Retry.Backoff = time.Duration(retryBackoff) * time.Millisecond

	return sarama.NewSyncProducer(brokers, config)
}

func NewConsumer(broker, group string) (sarama.ConsumerGroup, error) {
	brokers := []string{broker}
	if err := waitForKafka(brokers); err != nil {
		return nil, err
	}

	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Return.Errors = true

	return sarama.NewConsumerGroup(brokers, group, config)
}
