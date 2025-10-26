package main

import (
	"fmt"
	"log"
	//"os"
	//"os/signal"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/t6kke/learn-pub-sub-starter/internal/gamelogic"
	"github.com/t6kke/learn-pub-sub-starter/internal/routing"
	"github.com/t6kke/learn-pub-sub-starter/internal/pubsub"
)

func main() {
	fmt.Println("Starting Peril client...")

	const conn_url = "amqp://guest:guest@localhost:5672/"

	connection, err := amqp.Dial(conn_url)
	if err != nil {
		log.Printf("Failed to create connection to RabbitMQ: %v", err)
	}
	defer connection.Close()

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Printf("Welcome functionality failed: %v", err)
	}

	_, _, err = pubsub.DeclareAndBind(connection, routing.ExchangePerilDirect, routing.PauseKey+"."+username, routing.PauseKey, pubsub.SimpleQueueType(pubsub.Transient))
	if err != nil {
		log.Printf("Failed to register new channel and queue in RabbitMQ: %v", err)
	}

	game_state := gamelogic.NewGameState(username)

	err = pubsub.SubscribeJSON(connection, routing.ExchangePerilDirect, routing.PauseKey+"."+username, routing.PauseKey, pubsub.SimpleQueueType(pubsub.Transient), handlerPause(game_state))

	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}

		switch words[0] {
			case "spawn":
				err = game_state.CommandSpawn(words)
				if err != nil {
					fmt.Println(err)
					continue
				}
			case "move":
				_, err = game_state.CommandMove(words)
				if err != nil {
					fmt.Println(err)
					continue
				}
			case "status":
				game_state.CommandStatus()
			case "help":
				gamelogic.PrintClientHelp()
			case "spam":
				fmt.Println("Spamming not allowed yet!")
			case "quit":
				gamelogic.PrintQuit()
				return
			default:
				fmt.Printf("Did not reccodnize command: %s\n", words[0])
		}
	}

	// wait for ctrl+c
	//signalChan := make(chan os.Signal, 1)
	//signal.Notify(signalChan, os.Interrupt)
	//<-signalChan
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) {
	return func(ps routing.PlayingState) {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
	}
}
