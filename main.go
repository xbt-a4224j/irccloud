package main

import (
	"flag"
	"fmt"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
	"github.com/termoose/irccloud/config"
	"github.com/termoose/irccloud/events"
	"github.com/termoose/irccloud/requests"
	"github.com/termoose/irccloud/ui"
	"log"
	"os"
)

func main() {
	configFilename := flag.String("c", "", "path to config file")
	flag.Parse()

	// Set this, so we don't overwrite the default terminal
	// background color
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault

	var conf config.Data

	if isFlagSet("c") {
		conf = config.ParseCustom(*configFilename)
	} else {
		conf = config.Parse()
	}

	if conf.IsPlaceholder() {
		fmt.Fprintf(os.Stderr,
			"No credentials configured.\nEdit %s and set your IRCCloud username and password, then run this again.\n",
			configPath(*configFilename))
		os.Exit(1)
	}

	sessionData, err := requests.GetSessionToken(conf.Username, conf.Password)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	wsConn := requests.NewConnection(sessionData)
	view := ui.NewView(wsConn, &conf)

	defer func() {
		current := view.GetCurrentChannel()
		config.WriteLatestChannel(conf, current)
		view.Stop()
	}()

	eventHandler := events.NewHandler(sessionData.APIHost,
		sessionData.Session, view)

	go func() {
		for {
			msg, err := wsConn.ReadMessage()

			if err != nil {
				view.Stop()
				log.Print(err)

				return
			}

			eventHandler.Enqueue(msg)
		}
	}()

	view.Start()
}

func isFlagSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})

	return found
}

// configPath reports the file the user needs to edit, which differs when
// -c was passed.
func configPath(custom string) string {
	if custom != "" {
		return custom
	}

	return config.DefaultPath()
}
