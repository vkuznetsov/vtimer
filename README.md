## vtimer

Very simple cross-platform systray timer.
The time interval and time displaying format are passed by command line arguments.
Start and stop commands are available from the menu.

### Build and Usage

See [Platform Notes for getlantern/systray](https://github.com/getlantern/systray#platform-notes)

```
go build
./vtimer --help
```

```
Usage of vtimer:
  -display string
        Units for display remaining time. Supported values: "h" - hours only; "hm" - hours and minutes; "hms" - hours, minutes and seconds; "ms" - minutes and seconds; "s" - seconds only (default "ms")
  -interval string
        Timer interval. Ex: "25m", "1h5m14s". Supported units - h, m, s (default "25m")
  -no-state-symbols
        Do not use symbols for timer state and actions
  -state-symbols string
        Symbols for timer state and actions: restart, stop, continue (default "○□▷")
```