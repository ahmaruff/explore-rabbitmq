package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"rabbitmq-example/internal/platform/logger"

	rmq "github.com/rabbitmq/rabbitmq-amqp-go-client/pkg/rabbitmqamqp"
)

// closeTimeout bounds how long a Close call may take during shutdown.
const closeTimeout = 5 * time.Second

// NewConnection opens a connection and returns the environment that owns it.
func NewConnection(ctx context.Context, brokerURI string) (*rmq.Environment, *rmq.AmqpConnection, error) {
	env := rmq.NewEnvironment(brokerURI, nil)

	conn, err := env.NewConnection(ctx)
	if err != nil {
		return nil, nil, logger.WithStack(err)
	}

	return env, conn, nil
}

// CloseConnection closes every connection owned by the environment.
func CloseConnection(env *rmq.Environment) error {
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()

	if err := env.CloseConnections(ctx); err != nil {
		return logger.WithStack(err)
	}

	return nil
}

// NewQueue declares a quorum queue.
func NewQueue(ctx context.Context, conn *rmq.AmqpConnection, queueName string) (*rmq.AmqpQueueInfo, error) {
	queue, err := conn.Management().DeclareQueue(ctx, &rmq.QuorumQueueSpecification{Name: queueName})
	if err != nil {
		return nil, logger.WithStack(err)
	}

	return queue, nil
}

// NewPublisher creates a publisher that sends messages to the queue.
func NewPublisher(ctx context.Context, conn *rmq.AmqpConnection, queueName string) (*rmq.Publisher, error) {
	publisher, err := conn.NewPublisher(ctx, &rmq.QueueAddress{Queue: queueName}, nil)
	if err != nil {
		return nil, logger.WithStack(err)
	}

	return publisher, nil
}

// ClosePublisher closes the publisher.
func ClosePublisher(publisher *rmq.Publisher) error {
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()

	if err := publisher.Close(ctx); err != nil {
		return logger.WithStack(err)
	}

	return nil
}

// Publish sends payload to the publisher's queue. []byte and string are sent
// as-is, every other value is encoded as JSON.
func Publish(ctx context.Context, publisher *rmq.Publisher, payload any) error {
	var body []byte

	switch value := payload.(type) {
	case []byte:
		body = value
	case string:
		body = []byte(value)
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return logger.WithStack(fmt.Errorf("marshal message: %w", err))
		}

		body = encoded
	}

	res, err := publisher.Publish(ctx, rmq.NewMessage(body))
	if err != nil {
		return logger.WithStack(err)
	}

	switch res.Outcome.(type) {
	case *rmq.StateAccepted:
		logger.L().Info().Msg("[x] Sent")
		return nil
	case *rmq.StateRejected:
		if res.MessageRejectedError != nil {
			return logger.WithStack(fmt.Errorf("publish rejected by %q: %s", res.MessageRejectedError.RejectedBy, res.MessageRejectedError.Reason))
		}

		return logger.WithStack(fmt.Errorf("publish rejected"))
	default:
		return logger.WithStack(fmt.Errorf("unexpected publish outcome: %T", res.Outcome))
	}
}

// NewConsumer creates a consumer for the queue.
func NewConsumer(ctx context.Context, conn *rmq.AmqpConnection, queueName string) (*rmq.Consumer, error) {
	consumer, err := conn.NewConsumer(ctx, queueName, nil)
	if err != nil {
		return nil, logger.WithStack(err)
	}

	return consumer, nil
}

// CloseConsumer closes the consumer.
func CloseConsumer(consumer *rmq.Consumer) error {
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()

	if err := consumer.Close(ctx); err != nil {
		return logger.WithStack(err)
	}

	return nil
}

// Consume waits for messages until ctx is cancelled. The handler receives the
// raw message body and returns an error when the message could not be handled.
func Consume(ctx context.Context, consumer *rmq.Consumer, handler func(ctx context.Context, body []byte) error) error {
	for {
		delivery, err := consumer.Receive(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}

			return logger.WithStack(err)
		}

		msg := delivery.Message()

		var body []byte
		if len(msg.Data) > 0 {
			body = msg.Data[0]
		}

		if err := handler(ctx, body); err != nil {
			logger.L().Error().Stack().Err(err).Msg("Failed to handle message")

			// Put the message back so another attempt can be made.
			if requeueErr := delivery.Requeue(ctx); requeueErr != nil {
				return logger.WithStack(requeueErr)
			}

			continue
		}

		if err := delivery.Accept(ctx); err != nil {
			return logger.WithStack(err)
		}

		logger.L().Info().Msg("[x] Received")
	}
}
