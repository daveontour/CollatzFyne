package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {
	a := app.NewWithID("com.quaysystems.go.fyne.collatz")
	w := a.NewWindow("Collatz Visualisation")

	w.SetMaster()
	w.SetContent(makeEntryTab(w))

	// Create the go routines to update the UI
	go handleSingleModeStatusReport()
	go handleMultiModeStatusReport()

	w.Resize(fyne.NewSize(900, 800))
	w.ShowAndRun()
}
