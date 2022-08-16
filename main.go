package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/charmbracelet/gum/internal/exit"

	"golang.org/x/sys/windows"
)

const shaLen = 7

var (
	// Version contains the application version number. It's set via ldflags
	// when building.
	Version = ""

	// CommitSHA contains the SHA of the commit that this application was built
	// against. It's set via ldflags when building.
	CommitSHA = ""

	vtInputSupported bool
	consoleInMode    uint32
	consoleHandle    windows.Handle
)

var bubbleGumPink = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

func saveConsoleModeForPowerShell() {
	// get handle for console ... this took way to long to find how to get the console file stream ...
	//   from: https://github.com/FiloSottile/age/pull/274/files/03f6e59d5ccf931734067c5e1f06ac3baa1187cc#r643446247
	console, _ := os.OpenFile("CONIN$", os.O_RDWR, 0777)
	consoleHandle = windows.Handle(console.Fd())

	// consoleInMode is global var so we can revert after any gum stuff that changes it
	//   from: https://github.com/containerd/console/blob/main/console_windows.go#L34
	if err := windows.GetConsoleMode(consoleHandle, &consoleInMode); err == nil {
		// Validate that windows.ENABLE_VIRTUAL_TERMINAL_INPUT is supported, but do not set it.
		if err = windows.SetConsoleMode(consoleHandle, consoleInMode|windows.ENABLE_VIRTUAL_TERMINAL_INPUT); err == nil {
			vtInputSupported = true
		}
		// Unconditionally set the console mode back even on failure because SetConsoleMode
		//   remembers invalid bits on input handles.
		windows.SetConsoleMode(consoleHandle, consoleInMode)
	}
}

func revertConsoleModeForPowerShell() {
	console, err := os.OpenFile("CONIN$", os.O_RDWR, 0777)

	if err == nil {
		consoleHandle = windows.Handle(console.Fd())
		// use saved mode when starting the program and ensure
		//   ENABLE_VIRTUAL_TERMINAL_INPUT is enabled too
		if vtInputSupported {
			consoleInMode = consoleInMode | windows.ENABLE_VIRTUAL_TERMINAL_INPUT
		}
		windows.SetConsoleMode(consoleHandle, consoleInMode)
		// this what always worked for me in powershell, but saving the mode and
		//   reverting seems more sane
		// windows.SetConsoleMode(consoleHandle, 503)

		// close for extra measure?
		console.Close()
	}
}

func main() {
	// read and save the console mode
	saveConsoleModeForPowerShell()

	// always revert console mode after running
	defer revertConsoleModeForPowerShell()

	lipgloss.SetColorProfile(termenv.ANSI256)

	if Version == "" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Sum != "" {
			Version = info.Main.Version
		} else {
			Version = "unknown (built from source)"
		}
	}
	version := fmt.Sprintf("gum version %s", Version)
	if len(CommitSHA) >= shaLen {
		version += " (" + CommitSHA[:shaLen] + ")"
	}

	gum := &Gum{}
	ctx := kong.Parse(
		gum,
		kong.Description(fmt.Sprintf("A tool for %s shell scripts.", bubbleGumPink.Render("glamorous"))),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Summary: false,
		}),
		kong.Vars{
			"version":           version,
			"defaultBackground": "",
			"defaultForeground": "",
			"defaultMargin":     "0 0",
			"defaultPadding":    "0 0",
			"defaultUnderline":  "false",
		},
	)

	if err := ctx.Run(); err != nil {
		if errors.Is(err, exit.ErrAborted) {
			// read somewhere that `os.Exit()` means defers don't run, so
			//   calling here as well
			revertConsoleModeForPowerShell()
			os.Exit(exit.StatusAborted)
		}
		fmt.Println(err)
		revertConsoleModeForPowerShell()
		os.Exit(1)
	}
}
