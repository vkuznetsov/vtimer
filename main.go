package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/getlantern/systray"
	"github.com/pkg/errors"
)

type timer struct {
	display  displayFn
	interval time.Duration
	commands chan timerCommand
	events   chan timerEvent
	symbols  timerSymbolFn
}

type displayFn func(diff time.Duration) string
type timerCommand int
type timerEvent int
type timerSymbolCode int
type timerSymbolFn func(symbolCode timerSymbolCode) string

const (
	timerStopCommand timerCommand = iota
	timerContinueCommand
	timerRestartCommand
)

const (
	timerOutEvent timerEvent = iota
	timerPausedEvent
	timerStartedEvent
)

const (
	timerStopSymbol timerSymbolCode = iota
	timerContinueSymbol
	timerRestartSymbol
)

func main() {
	systray.Run(onReady, onExit)
}

func timerLoop(timer *timer) {
	var restInterval time.Duration
	var diff time.Duration

	started := true
	now := time.Now()
	stopTime := now.Add(timer.interval)

	timer.events <- timerStartedEvent

	for {
		now = time.Now()

		select {
		case cmd := <-timer.commands:
			switch cmd {
			case timerStopCommand:
				restInterval = stopTime.Sub(now)
				systray.SetTitle(timer.symbols(timerStopSymbol) + " " + timer.display(diff))
				timer.events <- timerPausedEvent
				started = false
			case timerContinueCommand:
				stopTime = now.Add(restInterval)
				timer.events <- timerStartedEvent
				started = true
			case timerRestartCommand:
				stopTime = now.Add(timer.interval)
				timer.events <- timerStartedEvent
				started = true
			}
		default:
		}

		if started {
			diff = stopTime.Sub(now)

			if diff > 0 {
				systray.SetTitle(timer.symbols(timerContinueSymbol) + " " + timer.display(diff))
			} else {
				started = false
				systray.SetTitle(timer.symbols(timerStopSymbol) + " " + timer.display(timer.interval))
				timer.events <- timerOutEvent
				notifyTimeout(timer)
			}
		}

		time.Sleep(time.Second)
	}
}

type menu struct {
	restartMenuItem  *systray.MenuItem
	stopMenuItem     *systray.MenuItem
	continueMenuItem *systray.MenuItem
	statsMenuItem    *systray.MenuItem
	quitMenuItem     *systray.MenuItem
}

func menuLoop(menu *menu, timerEvents chan timerEvent, timerCommands chan timerCommand) {
	intervalCounter := 0

	for {
		menu.statsMenuItem.SetTitle(fmt.Sprintf("%d intervals passed", intervalCounter))

		select {
		case timerEvent := <-timerEvents:
			switch timerEvent {
			case timerOutEvent:
				menu.continueMenuItem.Disable()
				menu.stopMenuItem.Disable()
				intervalCounter++
			case timerStartedEvent:
				menu.stopMenuItem.Enable()
				menu.continueMenuItem.Disable()
			case timerPausedEvent:
				menu.stopMenuItem.Disable()
				menu.continueMenuItem.Enable()
			}
		case <-menu.quitMenuItem.ClickedCh:
			systray.Quit()
		case <-menu.statsMenuItem.ClickedCh:
			intervalCounter = 0
		case <-menu.restartMenuItem.ClickedCh:
			timerCommands <- timerRestartCommand
		case <-menu.stopMenuItem.ClickedCh:
			timerCommands <- timerStopCommand
		case <-menu.continueMenuItem.ClickedCh:
			timerCommands <- timerContinueCommand
		}
	}
}

func onReady() {
	symbolsStr := flag.String("state-symbols", "○□▷", `Symbols for timer state and actions: restart, stop, continue`)
	noSymbols := flag.Bool("no-state-symbols", false, `Do not use symbols for timer state and actions`)
	intervalStr := flag.String("interval", "25m", `Timer interval. Ex: "25m", "1h5m14s". Supported units - h, m, s`)
	displayStr := flag.String("display", "ms", `Units for display remaining time. Supported values: `+
		`"h" - hours only; `+
		`"hm" - hours and minutes; `+
		`"hms" - hours, minutes and seconds; `+
		`"ms" - minutes and seconds; `+
		`"s" - seconds only`)

	flag.Parse()

	interval, err := parseInterval(*intervalStr)
	if err != nil {
		showHelpAndExit(err)
	}

	display, err := parseDisplay(*displayStr)
	if err != nil {
		showHelpAndExit(err)
	}

	symbols, err := parseTimerSymbols(*symbolsStr, *noSymbols)
	if err != nil {
		showHelpAndExit(err)
	}

	systray.SetTooltip(fmt.Sprintf("Timer set %s", *intervalStr))
	restartMenuItem := systray.AddMenuItem(symbols(timerRestartSymbol)+" Restart", "Restart timer")
	stopMenuItem := systray.AddMenuItem(symbols(timerStopSymbol)+" Stop", "Stop timer")
	continueMenuItem := systray.AddMenuItem(symbols(timerContinueSymbol)+" Continue", "Continue stopped timer")
	systray.AddSeparator()
	statsMenuItem := systray.AddMenuItem("", "Reset stats")
	quitMenuItem := systray.AddMenuItem("Quit", "Quit the whole app")

	menu := &menu{restartMenuItem, stopMenuItem, continueMenuItem, statsMenuItem, quitMenuItem}

	timerCommands := make(chan timerCommand)
	timerEvents := make(chan timerEvent)
	timer := &timer{interval: interval, display: display, commands: timerCommands, events: timerEvents, symbols: symbols}

	go menuLoop(menu, timerEvents, timerCommands)
	go timerLoop(timer)
}

func onExit() {
	// clean up here
}

func parseInterval(val string) (time.Duration, error) {
	re := regexp.MustCompile(`^(?P<hours>[0-9]+h)*\s*(?P<mins>[0-9]+m)*\s*(?P<secs>[0-9]+s)*$`)

	submatch := re.FindStringSubmatch(val)
	if submatch == nil {
		return 0, errors.New("invalid timer interval")
	}

	macthes := make(map[string]string)
	for i, name := range re.SubexpNames() {
		if i > 0 && i <= len(submatch) {
			macthes[name] = submatch[i]
		}
	}

	parseUnit := func(unitName string) int {
		strValue := macthes[unitName]
		if strValue == "" {
			return 0
		}

		value, _ := strconv.Atoi(strValue[:len(strValue)-1])
		return value
	}

	interval := time.Duration(parseUnit("hours")) * time.Hour
	interval += time.Duration(parseUnit("mins")) * time.Minute
	interval += time.Duration(parseUnit("secs")) * time.Second

	return interval, nil
}

func parseDisplay(val string) (fn displayFn, err error) {
	switch val {
	case "h":
		fn = func(diff time.Duration) string { return fmt.Sprintf("%dh", int(diff.Round(time.Hour).Hours())) }
	case "m":
		fn = func(diff time.Duration) string { return fmt.Sprintf("%dm", int(diff.Round(time.Minute).Minutes())) }
	case "s":
		fn = func(diff time.Duration) string { return fmt.Sprintf("%ds", int(diff.Round(time.Second).Seconds())) }
	case "hm":
		fn = func(diff time.Duration) string {
			mins := int(diff.Round(time.Minute).Minutes())
			hours := mins / 60
			mins = mins % 60

			return fmt.Sprintf("%02dh %02dm", hours, mins)
		}
	case "hms":
		fn = func(diff time.Duration) string {
			secs := int(diff.Round(time.Second).Seconds())
			mins := secs / 60
			hours := mins / 60
			mins = mins % 60
			secs = secs - mins*60 - hours*3600
			return fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)
		}
	case "ms":
		fn = func(diff time.Duration) string {
			secs := int(diff.Round(time.Second).Seconds())
			mins := secs / 60
			secs = secs % 60
			return fmt.Sprintf("%02d:%02d", mins, secs)
		}
	default:
		return nil, errors.New("invalid display argument")
	}

	return fn, nil
}

func parseTimerSymbols(symbols string, dontUseSymbols bool) (timerSymbolFn, error) {
	if dontUseSymbols {
		return func(symbolCode timerSymbolCode) string { return "" }, nil
	}

	runes := make([]string, 3)
	idx := 0
	for _, symbol := range symbols {
		if idx > 2 {
			return nil, errors.New("invalid symbols argument")
		}

		runes[idx] = string(symbol)
		idx++
	}

	res := func(symbolCode timerSymbolCode) string {
		switch symbolCode {
		case timerRestartSymbol:
			return runes[0]
		case timerStopSymbol:
			return runes[1]
		case timerContinueSymbol:
			return runes[2]
		default:
			panic(fmt.Sprintf("unexpected symbol requested %d", symbolCode))
		}
	}

	return res, nil
}

func showHelpAndExit(err error) {
	fmt.Println(err)
	fmt.Printf("Try %s --help\n", os.Args[0])
	systray.Quit()
}

func notifyTimeout(timer *timer) {
	msg := fmt.Sprintf("%s have passed", timer.display(timer.interval))
	if err := beeep.Notify("Time out", msg, ""); err != nil {
		panic(err)
	}
}
