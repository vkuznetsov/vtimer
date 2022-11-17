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
}

type displayFn func(diff time.Duration) string
type timerCommand int
type timerEvent int

const (
	timerStop timerCommand = iota
	timerContinue
	timerRestart
)

const (
	timeOut timerEvent = iota
)

func main() {
	systray.Run(onReady, onExit)
}

func startTimer(timer *timer) {
	counter := timer.interval
	started := true

	for {
		if started {
			if counter > 0 {
				systray.SetTitle(timer.display(counter))
				counter -= time.Second
			}
			if counter == 0 {
				started = false
				systray.SetTitle("timeout")
				notifyTimeout(timer)
			}
		}

		select {
		case cmd := <-timer.commands:
			switch cmd {
			case timerStop:
				started = false
			case timerContinue:
				started = true
			case timerRestart:
				counter = timer.interval
				started = true
			}
		default:
			time.Sleep(time.Second)
		}
	}
}

func onReady() {
	interval_str := flag.String("interval", "25m", `Timer interval. Ex: "25m", "1h5m14s". Supported units - h, m, s`)
	display_str := flag.String("display", "ms", `Units for display remaining time. Supported values: `+
		`"h" - hours only; `+
		`"hm" - hours and minutes; `+
		`"hms" - hours, minutes and seconds; `+
		`"ms" - minutes and seconds; `+
		`"s" - seconds only`)

	flag.Parse()

	interval, err := parseInterval(*interval_str)
	if err != nil {
		showHelpAndExit(err)
	}

	display, err := parseDisplay(*display_str)
	if err != nil {
		showHelpAndExit(err)
	}

	systray.SetTooltip(fmt.Sprintf("Timer set %s", *interval_str))
	restartMenuItem := systray.AddMenuItem("Restart", "Restart timer")
	stopMenuItem := systray.AddMenuItem("Stop", "Stop timer")
	continueMenuItem := systray.AddMenuItem("Continue", "Continue stopped timer")
	systray.AddSeparator()
	quitMenuItem := systray.AddMenuItem("Quit", "Quit the whole app")

	timerCommands := make(chan timerCommand)
	timerEvents := make(chan timerEvent)
	timer := timer{interval: interval, display: display, commands: timerCommands, events: timerEvents}

	go func() {
		timerStarted := true

		for {
			if timerStarted {
				stopMenuItem.Enable()
				continueMenuItem.Disable()
			} else {
				stopMenuItem.Disable()
				continueMenuItem.Enable()
			}

			select {
			case timerEvent := <-timerEvents:
				switch timerEvent {
				case timeOut:
					timerStarted = false
				}
			case <-quitMenuItem.ClickedCh:
				systray.Quit()
			case <-restartMenuItem.ClickedCh:
				timerCommands <- timerRestart
				timerStarted = true
			case <-stopMenuItem.ClickedCh:
				timerCommands <- timerStop
				timerStarted = false
			case <-continueMenuItem.ClickedCh:
				timerCommands <- timerContinue
				timerStarted = true
			}
		}
	}()

	go startTimer(&timer)
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
		str_value := macthes[unitName]
		if str_value == "" {
			return 0
		}

		value, _ := strconv.Atoi(str_value[:len(str_value)-1])
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
		fn = func(diff time.Duration) string { return fmt.Sprintf("%dh", int(diff.Hours())) }
	case "m":
		fn = func(diff time.Duration) string { return fmt.Sprintf("%dm", int(diff.Minutes())) }
	case "s":
		fn = func(diff time.Duration) string { return fmt.Sprintf("%ds", int(diff.Seconds())) }
	case "hm":
		fn = func(diff time.Duration) string {
			hours := int(diff.Hours())
			mins := int(diff.Minutes()) - hours*60
			return fmt.Sprintf("%02dh %02dm", hours, mins)
		}
	case "hms":
		fn = func(diff time.Duration) string {
			hours := int(diff.Hours())
			mins := int(diff.Minutes()) - hours*60
			secs := int(diff.Seconds()) - mins*60 - hours*3600
			return fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)
		}
	case "ms":
		fn = func(diff time.Duration) string {
			mins := int(diff.Minutes())
			secs := int(diff.Seconds()) - mins*60
			return fmt.Sprintf("%02d:%02d", mins, secs)
		}
	default:
		return nil, errors.New("invalid display argument")
	}

	return fn, nil
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
