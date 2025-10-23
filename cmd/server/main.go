package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/t6kke/learn-pub-sub-starter/internal/pubsub"
	"github.com/t6kke/learn-pub-sub-starter/internal/routing"
)

func main() {
	fmt.Println("Starting Peril server...")

	const conn_url = "amqp://guest:guest@localhost:5672/"

	connection, err := amqp.Dial(conn_url)
	if err != nil {
		log.Printf("Failed to create connection to RabbitMQ: %v", err)
	}
	defer connection.Close()

	fmt.Println("Connection successfully created")

	channel, err := connection.Channel()
	if err != nil {
		log.Printf("Failed to create channel: %v", err)
	}
	defer channel.Close()

	pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})

	// wait for ctrl+c
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan

	fmt.Println("Connection closed")
}
