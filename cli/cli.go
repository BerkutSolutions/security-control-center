package cli

import (
	"fmt"
	"os"
)

func Run() {
	if len(os.Args) < 2 {
		printCommands()
		return
	}
	switch os.Args[1] {
	case "migrate":
		runMigrate(os.Args[2:])
	case "create-user":
		runCreateUser(os.Args[2:])
	case "experiment":
		runExperiment(os.Args[2:])
	case "experiment-fit":
		runExperimentFit(os.Args[2:])
	default:
		fmt.Println("unknown command")
		printCommands()
	}
}

func printCommands() {
	fmt.Println("commands: migrate, create-user, experiment, experiment-fit")
}

