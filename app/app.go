package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/godbus/dbus/v5"
	"github.com/Sahas001/laya-tui/mpris"
	"github.com/Sahas001/laya-tui/ui"
)

// Version of the application
const Version = "1.0.0"

// WaybarOutput represents the structured JSON output for Waybar.
type WaybarOutput struct {
	Artist           string `json:"artist"`
	Title            string `json:"title"`
	Status           string `json:"status"`
	ProgressBarChars string `json:"progress_bar_chars"`
	Text             string `json:"text"`
	Alt              string `json:"alt"`
	Tooltip          string `json:"tooltip"`
	Class            string `json:"class"`
	Percentage       int    `json:"percentage"`
}

// Run parses command line flags, connects to D-Bus and starts the Bubble Tea loop
func Run() {
	listFlag := flag.Bool("list", false, "List active MPRIS media players and exit")
	playerFlag := flag.String("player", "", "Directly connect to a player by name (case-insensitive substring match)")
	streamFlag := flag.Bool("stream", false, "Continuously stream JSON metadata to stdout for status bars (e.g. Waybar)")
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

	if *streamFlag {
		runStream(conn, *playerFlag)
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

func generateProgressBarChars(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	filled := int(percent * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "00:00"
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func findStreamPlayer(conn *dbus.Conn, playerSearch string) (*mpris.Player, string) {
	players, err := mpris.ListPlayers(conn)
	if err != nil || len(players) == 0 {
		return nil, ""
	}

	if playerSearch != "" {
		searchTerm := strings.ToLower(playerSearch)
		for _, name := range players {
			identity, _ := mpris.GetPlayerIdentity(conn, name)
			if strings.Contains(strings.ToLower(identity), searchTerm) ||
				strings.Contains(strings.ToLower(name), searchTerm) {
				return mpris.NewPlayer(conn, name), identity
			}
		}
		return nil, ""
	}

	// Try to find one that is playing
	var firstPlayer *mpris.Player
	var firstIdentity string
	for _, name := range players {
		p := mpris.NewPlayer(conn, name)
		state, err := p.GetState()
		identity, _ := mpris.GetPlayerIdentity(conn, name)
		if err == nil {
			if state.PlaybackStatus == "Playing" {
				return p, identity
			}
			if firstPlayer == nil {
				firstPlayer = p
				firstIdentity = identity
			}
		}
	}

	return firstPlayer, firstIdentity
}

func runStream(conn *dbus.Conn, playerSearch string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var player *mpris.Player
	var identity string

	for {
		if player == nil {
			player, identity = findStreamPlayer(conn, playerSearch)
		}

		var output WaybarOutput
		if player == nil {
			output = WaybarOutput{
				Status: "Stopped",
				Text:   "No active player",
				Alt:    "stopped",
				Class:  "stopped",
			}
		} else {
			state, err := player.GetState()
			if err != nil {
				// Player disconnected/exited
				player = nil
				identity = ""
				continue
			}

			// Interpolate position
			pos := state.Position
			if state.PlaybackStatus == "Playing" {
				elapsed := time.Since(state.LastUpdated)
				pos += elapsed
				if state.Metadata.Length > 0 && pos > state.Metadata.Length {
					pos = state.Metadata.Length
				}
			}

			var percent float64
			var pct int
			if state.Metadata.Length > 0 {
				percent = float64(pos) / float64(state.Metadata.Length)
				if percent > 1.0 {
					percent = 1.0
				}
				pct = int(percent * 100)
			}

			bar := generateProgressBarChars(percent, 10)

			title := state.Metadata.Title
			if title == "" {
				title = "Unknown Track"
			}
			artist := state.Metadata.Artist
			if artist == "" {
				artist = "Unknown Artist"
			}

			var text string
			if state.PlaybackStatus == "Playing" {
				text = fmt.Sprintf("%s - %s", artist, title)
			} else if state.PlaybackStatus == "Paused" {
				text = fmt.Sprintf("⏸ %s - %s", artist, title)
			} else {
				text = fmt.Sprintf("⏹ %s - %s", artist, title)
			}

			tooltip := fmt.Sprintf("Title: %s\nArtist: %s\nAlbum: %s\nStatus: %s\nPlayer: %s", 
				title, artist, state.Metadata.Album, state.PlaybackStatus, identity)
			if state.Metadata.Length > 0 {
				tooltip += fmt.Sprintf("\nProgress: %s / %s (%d%%)\n%s", 
					formatDuration(pos), 
					formatDuration(state.Metadata.Length), 
					pct, 
					bar,
				)
			}

			output = WaybarOutput{
				Artist:           artist,
				Title:            title,
				Status:           state.PlaybackStatus,
				ProgressBarChars: bar,
				Text:             text,
				Alt:              strings.ToLower(state.PlaybackStatus),
				Tooltip:          tooltip,
				Class:            strings.ToLower(state.PlaybackStatus),
				Percentage:       pct,
			}
		}

		jsonData, err := json.Marshal(output)
		if err == nil {
			fmt.Println(string(jsonData))
		}

		<-ticker.C
	}
}
