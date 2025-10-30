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

	channel, err := connection.Channel()
	//TODO error handling

	_, _, err = pubsub.DeclareAndBind(connection, routing.ExchangePerilDirect, routing.PauseKey+"."+username, routing.PauseKey, pubsub.SimpleQueueType(pubsub.Transient))
	if err != nil {
		log.Printf("Failed to register new channel and queue in RabbitMQ: %v", err)
	}

	game_state := gamelogic.NewGameState(username)

	err = pubsub.SubscribeJSON(connection, routing.ExchangePerilDirect, routing.PauseKey+"."+username, routing.PauseKey, pubsub.SimpleQueueType(pubsub.Transient), handlerPause(game_state))
	//TODO error handling

	err = pubsub.SubscribeJSON(connection, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+username, routing.ArmyMovesPrefix+".*", pubsub.SimpleQueueType(pubsub.Transient), handlerMove(game_state, channel))
	//TODO error handling

	err = pubsub.SubscribeJSON(connection, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, routing.WarRecognitionsPrefix+".*", pubsub.SimpleQueueType(pubsub.Durable), handlerWar(game_state))
	//TODO error handling

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
				army_move, err := game_state.CommandMove(words)
				if err != nil {
					fmt.Println(err)
					continue
				}
				err = pubsub.PublishJSON(channel, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+username, army_move)
				if err != nil {
					fmt.Println(err)
					continue
				}
				fmt.Println("Move published successfully")
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

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.Acktype {
	return func(ps routing.PlayingState) pubsub.Acktype {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.ArmyMove) pubsub.Acktype {
	return func(am gamelogic.ArmyMove) pubsub.Acktype {
		defer fmt.Print("> ")
		move_outcome := gs.HandleMove(am)
		switch move_outcome {
			case gamelogic.MoveOutComeSafe:
				return pubsub.Ack
			case gamelogic.MoveOutcomeMakeWar:
				payload := gamelogic.RecognitionOfWar{
					Attacker: am.Player,
					Defender: gs.GetPlayerSnap(),
				}
				err := pubsub.PublishJSON(ch, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix+"."+gs.GetUsername(), payload)
				if err != nil {
					return pubsub.NackRequeue
				}
				return pubsub.Ack
			case gamelogic.MoveOutcomeSamePlayer:
				return pubsub.NackDiscard
			default:
				return pubsub.NackDiscard
		}
	}
}

func handlerWar(gs *gamelogic.GameState) func(gamelogic.RecognitionOfWar) pubsub.Acktype {
	return func(rw gamelogic.RecognitionOfWar) pubsub.Acktype {
		defer fmt.Print("> ")
		outcome, _, _ := gs.HandleWar(rw)
		switch outcome {
			case gamelogic.WarOutcomeNotInvolved:
				return pubsub.NackRequeue
			case gamelogic.WarOutcomeNoUnits:
				return pubsub.NackDiscard
			case gamelogic.WarOutcomeOpponentWon:
				return pubsub.Ack
			case gamelogic.WarOutcomeYouWon:
				return pubsub.Ack
			case gamelogic.WarOutcomeDraw:
				return pubsub.Ack
			default:
				fmt.Println("not a valid outcome")
				return pubsub.NackDiscard
		}
	}
}
