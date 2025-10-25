package main

import (
	"fmt"
	"log"
	//"os"
	//"os/signal"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/t6kke/learn-pub-sub-starter/internal/gamelogic"
	"github.com/t6kke/learn-pub-sub-starter/internal/pubsub"
	"github.com/t6kke/learn-pub-sub-starter/internal/routing"
)

func main() {
	fmt.Println("Starting Peril server...")

	const conn_url = "amqp://guest:guest@localhost:5672/"

	connection, err := amqp.Dial(conn_url)
	if err != nil {
		log.Printf("Failed to create connection to RabbitMQ: %v\n", err)
	}
	defer connection.Close()

	fmt.Println("Connection successfully created")

	gamelogic.PrintServerHelp()

	forLoop:for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}

		var pause_var bool
		switch words[0] {
			case "pause":
				log.Println("Pausing the game")
				pause_var = true
			case "resume":
				log.Println("Resuming the game")
				pause_var = false
			case "quit":
				log.Println("Closing Server")
				break forLoop
			default:
				log.Printf("Did not reccodnize command: %s\n", words[0])
		}

		channel, err := connection.Channel()
		if err != nil {
			log.Printf("Failed to create channel: %v\n", err)
		}

		pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: pause_var})
	}

	// wait for ctrl+c
	//signalChan := make(chan os.Signal, 1)
	//signal.Notify(signalChan, os.Interrupt)
	//<-signalChan
	//fmt.Println("Connection closed")
}
