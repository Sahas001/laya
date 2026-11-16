// Package app wires command-line flags, D-Bus and the Bubble Tea program together.
package app

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/Sahas001/laya/internal/audio"
	"github.com/Sahas001/laya/internal/notify"
	"github.com/Sahas001/laya/mpris"
	"github.com/Sahas001/laya/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/godbus/dbus/v5"
)

// Version is set at build time with -ldflags "-X github.com/Sahas001/laya/app.Version=…".
var Version = ""

func version() string {
	if Version != "" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// Run parses command line flags, connects to D-Bus and starts the Bubble Tea loop.
func Run() {
	listFlag := flag.Bool("list", false, "List active MPRIS media players and exit")
	playerFlag := flag.String("player", "", "Connect to a player by name (case-insensitive substring match)")
	streamFlag := flag.Bool("stream", false, "Stream JSON metadata to stdout for status bars (e.g. Waybar)")
	formatFlag := flag.String("format", "", "With --stream: emit plain text from a template instead of JSON\n"+
		"(placeholders: {artist} {title} {album} {player} {status} {icon}\n {position} {length} {percentage} {bar} {text})")
	intervalFlag := flag.Duration("interval", time.Second, "With --stream: refresh interval")
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	noAudioFlag := flag.Bool("no-audio", false, "Don't capture system audio; use the synthetic visualizer")
	artFlag := flag.String("art", "auto", "Album art: auto, kitty, blocks or off")
	vizFlag := flag.String("viz", "bars", "Initial visualizer style: bars, mirror, scope or braille")
	notifyFlag := flag.Bool("notify", false, "Show a desktop notification when the track changes")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Laya — MPRIS media controller & now-playing dashboard\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  laya-tui [flags]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  laya-tui --player spotify\n")
		fmt.Fprintf(os.Stderr, "  laya-tui --viz mirror --notify\n")
		fmt.Fprintf(os.Stderr, "  laya-tui --stream                                        # Waybar JSON\n")
		fmt.Fprintf(os.Stderr, "  laya-tui --stream --format '{icon} {artist} - {title}'   # Polybar text\n")
	}
	flag.Parse()

	artMode, err := ui.ParseArtMode(*artFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	if *versionFlag {
		fmt.Printf("laya-tui %s\n", version())
		return
	}

	conn, err := dbus.SessionBus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to the D-Bus session bus: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure you are running a Linux desktop session with D-Bus active.\n")
		os.Exit(1)
	}

	if *listFlag {
		listPlayers(conn)
		return
	}

	if *streamFlag {
		opts := StreamOptions{
			PlayerSearch: *playerFlag,
			Format:       *formatFlag,
			Interval:     *intervalFlag,
		}
		if *notifyFlag {
			opts.Notifier = notify.New(conn)
		}
		runStream(conn, opts, os.Stdout)
		return
	}

	m := ui.NewModel(conn)
	term := ui.NewTerminal(os.Stdout)
	m.SetTerminal(term)
	m.SetArtMode(artMode)
	m.SetVizStyle(ui.ParseVizStyle(*vizFlag))
	m.SetAudio(audio.New(), !*noAudioFlag)
	if *notifyFlag {
		m.SetNotifier(notify.New(conn))
	}

	if *playerFlag != "" {
		names, err := mpris.ListPlayers(conn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing players: %v\n", err)
			os.Exit(1)
		}
		bus, identity, ok := mpris.MatchPlayer(conn, names, *playerFlag)
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: no active media player matches %q.\n", *playerFlag)
			fmt.Fprintf(os.Stderr, "Run with --list to see available players.\n")
			os.Exit(1)
		}
		m.SetInitialPlayer(bus, identity)
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithOutput(term))

	// Forward MPRIS change signals so the UI reacts instantly instead of waiting
	// for the next poll.
	if events, stop, err := mpris.Watch(); err == nil {
		defer stop()
		go func() {
			for ev := range events {
				p.Send(ui.BusEventMsg{PlayersChanged: ev.Kind == mpris.EventPlayersChanged})
			}
		}()
	}

	_, err = p.Run()
	m.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func listPlayers(conn *dbus.Conn) {
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
	for _, name := range players {
		identity, _ := mpris.GetPlayerIdentity(conn, name)
		status := "?"
		track := ""
		if st, err := mpris.NewPlayer(conn, name).GetState(); err == nil {
			status = st.PlaybackStatus
			if st.Metadata.Title != "" {
				track = "  — " + st.Metadata.Title
				if st.Metadata.Artist != "" {
					track += " · " + st.Metadata.Artist
				}
			}
		}
		fmt.Printf("  • %-36s %-8s %s%s\n", identity, status, mpris.ShortName(name), track)
	}
}
