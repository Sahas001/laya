package app

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/godbus/dbus/v5"
	"github.com/Sahas001/laya-tui/mpris"
	"github.com/Sahas001/laya-tui/ui"
)

// Version of the application
const Version = "1.0.0"

// Run parses command line flags, connects to D-Bus and starts the Bubble Tea loop
func Run() {
	listFlag := flag.Bool("list", false, "List active MPRIS media players and exit")
	playerFlag := flag.String("player", "", "Directly connect to a player by name (case-insensitive substring match)")
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	
	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Laya TUI - Distributable MPRIS Media Control Dashboard\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  laya-tui [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Laya TUI v%s\n", Version)
		return
	}

	conn, err := dbus.SessionBus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Cannot connect to D-Bus Session Bus: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure you are running a Linux desktop environment with D-Bus active.\n")
		os.Exit(1)
	}

	if *listFlag {
		players, err := mpris.ListPlayers(conn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing media players: %v\n", err)
			os.Exit(1)
		}
		if len(players) == 0 {
			fmt.Println("No active MPRIS media players detected.")
			return
		}
		fmt.Println("Active MPRIS media players:")
		for _, p := range players {
			identity, _ := mpris.GetPlayerIdentity(conn, p)
			fmt.Printf("  • %s (Bus: %s)\n", identity, p)
		}
		return
	}

	// Create UI model
	m, err := ui.NewModel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing TUI: %v\n", err)
		os.Exit(1)
	}

	// If player flag was specified, attempt direct match
	if *playerFlag != "" {
		players, err := mpris.ListPlayers(conn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing players: %v\n", err)
			os.Exit(1)
		}

		var matchedBusName string
		var matchedIdentity string
		searchTerm := strings.ToLower(*playerFlag)
		
		for _, name := range players {
			identity, _ := mpris.GetPlayerIdentity(conn, name)
			if strings.Contains(strings.ToLower(identity), searchTerm) ||
				strings.Contains(strings.ToLower(name), searchTerm) {
				matchedBusName = name
				matchedIdentity = identity
				break
			}
		}

		if matchedBusName != "" {
			m.SetInitialPlayer(matchedBusName, matchedIdentity)
		} else {
			fmt.Fprintf(os.Stderr, "Error: No active media player found matching search term: %q\n", *playerFlag)
			fmt.Fprintf(os.Stderr, "Run with --list to view all available players.\n")
			os.Exit(1)
		}
	}

	// Run bubbletea program using alternate screen buffer & mouse cell motion
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
