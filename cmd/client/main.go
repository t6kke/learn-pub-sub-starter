package main

import (
	"fmt"
	"log"
	"time"
	"strconv"
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

	err = pubsub.SubscribeJSON(connection, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, routing.WarRecognitionsPrefix+".*", pubsub.SimpleQueueType(pubsub.Durable), handlerWar(game_state, channel))
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
				if len(words) < 2 {
					fmt.Println("Please provide a number value")
					continue
				}
				spam_count, err := strconv.Atoi(words[1])
				if err != nil {
					fmt.Println("Not a valid number provided")
					continue
				}
				for i := 0; i <= spam_count; i++ {
					message := gamelogic.GetMaliciousLog()
					publisGameLog(channel, message, username)
				}
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

func handlerWar(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.Acktype {
	return func(rw gamelogic.RecognitionOfWar) pubsub.Acktype {
		defer fmt.Print("> ")
		outcome, winner, loser := gs.HandleWar(rw)
		var result_acktype pubsub.Acktype
		var win_log_message string
		switch outcome {
			case gamelogic.WarOutcomeNotInvolved:
				result_acktype = pubsub.NackRequeue
			case gamelogic.WarOutcomeNoUnits:
				result_acktype = pubsub.NackDiscard
			case gamelogic.WarOutcomeOpponentWon:
				result_acktype = pubsub.Ack
				win_log_message = fmt.Sprintf("%s won a war against %s", winner, loser)
			case gamelogic.WarOutcomeYouWon:
				result_acktype = pubsub.Ack
				win_log_message = fmt.Sprintf("%s won a war against %s", winner, loser)
			case gamelogic.WarOutcomeDraw:
				result_acktype = pubsub.Ack
				win_log_message = fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
			default:
				fmt.Println("not a valid outcome")
				result_acktype = pubsub.NackDiscard
		}
		err := publisGameLog(ch, win_log_message, gs.GetUsername())
		if err != nil {
			result_acktype = pubsub.NackRequeue
		} else {
			result_acktype = pubsub.Ack
		}

		return result_acktype
	}
}

func publisGameLog(ch *amqp.Channel, message, username string) error {
	data := routing.GameLog{
		CurrentTime: time.Now(),
		Message:     message,
		Username:    username,
	}
	err := pubsub.PublishGob(ch, routing.ExchangePerilTopic, routing.GameLogSlug+"."+username, data)
	if err != nil {
		return err
	}
	return nil
}
