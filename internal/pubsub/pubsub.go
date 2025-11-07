package pubsub

import (
	//"log"
	"bytes"
	"context"
	"encoding/json"
	"encoding/gob"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType int

const (
	Durable   SimpleQueueType = iota
	Transient SimpleQueueType = iota
)

type Acktype int

const (
	Ack         Acktype = iota
	NackRequeue Acktype = iota
	NackDiscard Acktype = iota
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	jsonData, err := json.Marshal(val)
	if err != nil {
		return err
	}

	err = ch.PublishWithContext(context.Background(), exchange, key, false,false, amqp.Publishing{ContentType: "application/json", Body: jsonData})
	if err != nil {
		return err
	}

	return nil
}


func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(val)
	if err != nil {
		return err
	}

	err = ch.PublishWithContext(context.Background(), exchange, key, false,false, amqp.Publishing{ContentType: "application/gob", Body: buffer.Bytes()})
	if err != nil {
		return err
	}

	return nil
}


func DeclareAndBind(conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType) (*amqp.Channel, amqp.Queue, error) {
	new_channel, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	var durable bool
	var autoDelete bool
	var exclusive bool

	switch queueType {
		case Durable:
			durable = true
			autoDelete = false
			exclusive = false
		case Transient:
			durable = false
			autoDelete = true
			exclusive = true
	}

	data_table := amqp.Table{"x-dead-letter-exchange": "peril_dlx"}

	new_queue, err := new_channel.QueueDeclare(queueName, durable, autoDelete, exclusive, false, data_table)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	err = new_channel.QueueBind(queueName, key, exchange, false, nil)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	return new_channel, new_queue, nil
}


func SubscribeJSON[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T) Acktype) error {
	channel, queue, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}

	err = channel.Qos(10,0,false)
	if err != nil {
		return err
	}

	new_chan, err := channel.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		defer channel.Close()
		for message := range new_chan {
			var data T
			_ = json.Unmarshal(message.Body, &data)
			ack_type := handler(data)
			switch ack_type {
				case Acktype(Ack):
					//log.Printf("Ack call\n")
					message.Ack(false)
				case Acktype(NackRequeue):
					//log.Printf("Nack requeue call\n")
					message.Nack(false, true)
				case Acktype(NackDiscard):
					//log.Printf("Nack discard call\n")
					message.Nack(false, false)
			}

		}
	}()
	return nil
}

func SubscribeGob[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T) Acktype) error {
	channel, queue, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}

	err = channel.Qos(10,0,false)
	if err != nil {
		return err
	}

	new_chan, err := channel.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		defer channel.Close()
		for message := range new_chan {
			buffer := bytes.NewBuffer(message.Body)
			decoder := gob.NewDecoder(buffer)

			var data T
			err := decoder.Decode(&data)
			if err != nil {
				return
			}

			ack_type := handler(data)
			switch ack_type {
				case Acktype(Ack):
					//log.Printf("Ack call\n")
					message.Ack(false)
				case Acktype(NackRequeue):
					//log.Printf("Nack requeue call\n")
					message.Nack(false, true)
				case Acktype(NackDiscard):
					//log.Printf("Nack discard call\n")
					message.Nack(false, false)
			}

		}
	}()
	return nil
}
