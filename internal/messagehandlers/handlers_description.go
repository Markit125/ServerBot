package messagehandlers

import (
	"os"
)

var (
	handlersDescription string = "default handlers description"
)

func ReadHandlersDescriptionText() {
	textBytes, err := os.ReadFile("ServerBot/resources/handlers_description.txt")
	if err != nil {
		panic("cannot read file with handlers description: " + err.Error())
	}

	handlersDescription = string(textBytes)
}

func handlersDescriptionText() string {
	return handlersDescription
}
