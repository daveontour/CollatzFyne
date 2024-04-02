package main

import (
	//"fyne.io/fyne/v2/data/validation"
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

type sequenceProgress struct {
	stones         []float64
	stonesRaw      []float64
	stonesString   []string
	upwards        []bool
	steps          int
	upMoves        int
	downMoves      int
	maxStoneFloat  *big.Float
	maxStoneInt    *big.Int
	maxStoneString string
	lastStone      bool
	number         *big.Int
}

type collatzWorker struct {
	workerID        int
	finishedChannel chan bool
}

var zeroBig = big.NewInt(0)
var oneBig = big.NewInt(1)
var twoBig = big.NewInt(2)
var threeBig = big.NewInt(3)
var highmaxStone = big.NewInt(0)

// The channels for the UI
var sequneceStatusChannel = make(chan sequenceProgress)
var sequneceStatusPerfChannel = make(chan sequenceProgress, 100000)
var pauseChannel = make(chan bool)
var stopChannel = make(chan bool)
var stepChannel = make(chan bool)
var resumeChannel = make(chan bool)

// The buttons for the UI
var calcSingleBtn *widget.Button
var calcBtn *widget.Button
var pauseBtn *widget.Button
var stepBtn *widget.Button
var resumeBtn *widget.Button
var stopBtn *widget.Button

// UI elements for the stones
var highwaterStoneLabel *widget.Label
var highwaterStoneNumberLabel *widget.Label
var highwaterStepsLabel *widget.Label
var highwaterStepsNumberLabel *widget.Label
var upDownPercentageLabel *widget.Label

// UI elements for the summary
var number *widget.Label
var numUp *widget.Label
var numDown *widget.Label
var maxStone *widget.Label
var seqLen *widget.Label

var detailStoneList *widget.Table
var stoneStrings []string
var upDirectionBool []bool

var highwaterSteps int = 0
var highwaterStepsNumber string
var highwaterStone string
var highwaterStoneNumber string

var stepsSlice []float64
var stepsNumberSlice []float64

var progress *widget.ProgressBar
var infProgress *widget.ProgressBarInfinite

var workersDispatched int = 0
var workersFinished int = 0

var reportFreqencyInterval int = 1000

// The objects holding the graphs
var stonesChart *fyne.Container
var stonesLogChart *fyne.Container
var sequenceLengthChart *fyne.Container

var workDistributorChannel = make(chan big.Int)
var wg sync.WaitGroup

func (w *collatzWorker) Start(workerID int) {

	//The worker will listen to the workDistributorChannel and process the values
	//The worker will also listen to its own finishedChannel and stop processing when it receives a value
	//The worker will also keep track of the number of values it has processed

	w.finishedChannel = make(chan bool)
	w.workerID = workerID

	handled := 0
	go func() {
		for {
			select {
			case value := <-workDistributorChannel:
				handled++
				report := CollatzPerf(value)
				sequneceStatusPerfChannel <- report
				wg.Done()
			case <-w.finishedChannel:
				//fmt.Printf("Process %d handled %d values\n", w.workerID, handled)
				wg.Done()
			}
		}
	}()
}

type threadSafeSlice struct {
	sync.Mutex
	workers []*collatzWorker
}

func (slice *threadSafeSlice) Push(w *collatzWorker) {
	slice.Lock()
	defer slice.Unlock()

	slice.workers = append(slice.workers, w)
}

func (slice *threadSafeSlice) Iter(routine func(*collatzWorker)) {
	slice.Lock()
	defer slice.Unlock()

	for _, worker := range slice.workers {
		routine(worker)
	}
}

var workersThreadSafeSlice = &threadSafeSlice{}

func makeEntryTab(win fyne.Window) fyne.CanvasObject {

	tabs := container.NewAppTabs(
		container.NewTabItem("Single Value", makeSingleTab(win)),
		container.NewTabItem("Range", makeMultiTab(win)),
	)
	return container.NewBorder(widget.NewLabel("Collatz Conjecture Visualiser"), nil, nil, nil, tabs)
}

func makeSingleTab(win fyne.Window) fyne.CanvasObject {

	// The graph for the absolute values
	stonesChart = container.NewStack()

	// The graph for the log values
	stonesLogChart = container.NewStack()

	sequenceLengthChart = container.NewStack()

	// The list of stones
	detailStoneList = widget.NewTable(
		func() (int, int) {
			if len(stoneStrings) == 0 {
				return 3, 3
			}
			return len(stoneStrings) + 1, 3
		},
		func() fyne.CanvasObject {
			item := widget.NewLabel("Template Wide Label")
			item.Resize(fyne.Size{
				Width:  400,
				Height: 20,
			})
			return item
		},
		func(i widget.TableCellID, cell fyne.CanvasObject) {

			label := cell.(*widget.Label)

			if i.Row == 0 && i.Col == 0 {
				label.SetText("Sequence Number")
				return
			}
			if i.Row == 0 && i.Col == 1 {
				label.SetText("Hailstone")
				return
			}
			if i.Row == 0 && i.Col == 2 {
				label.SetText("Direction")
				return
			}

			if len(stoneStrings) != 0 {
				if i.Col == 0 {
					label.SetText(fmt.Sprintf("%d", i.Row))
					return
				}
				if i.Col == 1 {
					label.SetText(stoneStrings[i.Row-1])
					return
				}
				if i.Col == 2 && i.Row > 1 {
					if upDirectionBool[i.Row-1] {
						label.SetText("Up")
					} else {
						label.SetText("Down")
					}
					return
				}
				if i.Col == 2 {
					label.SetText("-")
					return
				}
			} else {
				label.SetText("")
				return
			}
		})
	detailStoneList.ShowHeaderColumn = false
	detailStoneList.ShowHeaderRow = false
	detailStoneList.StickyRowCount = 1
	detailStoneList.Resize(fyne.NewSize(600, 400))
	tableLayout := container.NewStack(detailStoneList)
	tableLayout.Resize(fyne.NewSize(400, 400))

	// The summary tab
	number = widget.NewLabel("")
	numUp = widget.NewLabel("")
	numDown = widget.NewLabel("")
	maxStone = widget.NewLabel("")
	seqLen = widget.NewLabel("")
	upDownPercentageLabel = widget.NewLabel("")

	summary := container.NewHBox(
		container.NewVBox(
			widget.NewRichTextFromMarkdown("**Number**"),
			widget.NewRichTextFromMarkdown("**Sequence Length**"),
			widget.NewRichTextFromMarkdown("**Max Stone for Sequence**"),
			widget.NewRichTextFromMarkdown("**Number of Upwards**"),
			widget.NewRichTextFromMarkdown("**Number of Downwards**"),
			widget.NewRichTextFromMarkdown("**Up/Down Percentage**"),
		),
		container.NewVBox(
			number,
			seqLen,
			maxStone,
			numUp,
			numDown,
			upDownPercentageLabel,
		),
	)

	// Put the elements into a tab set
	statusTabs := container.NewAppTabs(
		container.NewTabItem("Summary", summary),
		container.NewTabItem("Absolute Hailstone Chart", stonesChart),
		container.NewTabItem("Log Hailstone Chart", stonesLogChart),
		container.NewTabItem("Details", tableLayout),
	)

	//create the left pane and put the two together into a split
	splitCanvas := container.NewHSplit(
		makeLeftPaneSingle(win),
		container.NewBorder(nil, nil, nil, nil, statusTabs),
	)
	splitCanvas.Offset = 0.4

	return splitCanvas
}
func makeLeftPaneSingle(win fyne.Window) fyne.CanvasObject {

	var entryValue *widget.Entry

	entryBase := widget.NewSelect([]string{"Base 2", "Base 10", "Base 16", "Base 36"}, func(string) {})
	entryBase.SetSelected("Base 10")
	entryBase.PlaceHolder = "Select a base"
	entryBase.OnChanged = func(s string) {
		checkValidation(entryValue.Text, s, win)
	}
	entryValue = widget.NewEntry()
	entryValue.SetPlaceHolder("")
	entryValue.OnChanged = func(s string) {
		s = removeSpaces(s)
		checkValidation(s, entryBase.Selected, win)
		entryValue.SetText(s)
	}

	calcSingleBtn = widget.NewButton("Calculate", func() {
		go calcStones(entryValue.Text, entryBase.Selected, win)
	})
	calcSingleBtn.Enable()

	return container.NewBorder(container.NewVBox(
		widget.NewForm(
			widget.NewFormItem(fmt.Sprintf("%15s", "Entry Base:"), entryBase),
			widget.NewFormItem(fmt.Sprintf("%15s", "Value:"), entryValue),
		)), calcSingleBtn, nil, nil, nil)
}
func makeMultiTab(win fyne.Window) fyne.CanvasObject {

	sequenceLengthChart = container.NewStack()

	// The summary tab

	highwaterStoneLabel = widget.NewLabel("")
	highwaterStoneNumberLabel = widget.NewLabel("")
	highwaterStepsLabel = widget.NewLabel("")
	highwaterStepsNumberLabel = widget.NewLabel("")

	summary := container.NewHBox(
		container.NewVBox(
			widget.NewRichTextFromMarkdown("**Max Sequence Length**"),
			widget.NewRichTextFromMarkdown("**Max Sequence Length Number**"),
			widget.NewRichTextFromMarkdown("**Max Stone**"),
			widget.NewRichTextFromMarkdown("**Max Stone Number**")),
		container.NewVBox(
			highwaterStepsLabel,
			highwaterStepsNumberLabel,
			highwaterStoneLabel,
			highwaterStoneNumberLabel,
		),
	)

	// Put the elements into a tab set
	statusTabs := container.NewAppTabs(
		container.NewTabItem("High Water Marks", summary),
		container.NewTabItem("Sequence Length Chart", sequenceLengthChart),
	)

	// Put the tab set into a border layout at the bottop
	border := container.NewBorder(nil, nil, nil, nil, statusTabs)

	//create the left pane and put the two together into a split
	splitCanvas := container.NewHSplit(makeLeftPaneMulti(win), border)
	splitCanvas.Offset = 0.4

	return splitCanvas
}
func makeLeftPaneMulti(win fyne.Window) fyne.CanvasObject {

	var entryLower *widget.Entry
	var entryUpper *widget.Entry
	var reportFreq *widget.Entry

	entryBase := widget.NewSelect([]string{"Base 2", "Base 10", "Base 16", "Base 36"}, func(string) {})
	entryBase.SetSelected("Base 10")
	entryBase.PlaceHolder = "Select a base"
	entryBase.OnChanged = func(s string) {
		checkValidation(entryLower.Text, s, win)
	}

	entryLower = widget.NewEntry()
	entryLower.SetPlaceHolder("")
	entryLower.OnChanged = func(s string) {
		s = removeSpaces(s)
		checkValidation(s, entryBase.Selected, win)
		entryLower.SetText(s)
	}
	entryUpper = widget.NewEntry()
	entryUpper.SetPlaceHolder("")
	entryUpper.OnChanged = func(s string) {
		s = removeSpaces(s)
		checkValidation(s, entryBase.Selected, win)
		entryUpper.SetText(s)
	}

	reportFreq = widget.NewEntry()
	reportFreq.SetPlaceHolder("1000")
	reportFreq.OnChanged = func(s string) {
		s = removeSpaces(s)
		v, ok := checkValidation(s, "10", win)
		reportFreq.SetText(s)
		if ok {
			reportFreqencyInterval = int(v.Int64())
		}
	}

	fixed := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem(fmt.Sprintf("%15s", "Entry Base:"), entryBase),
			widget.NewFormItem(fmt.Sprintf("%15s", "Lower Limit:"), entryLower),
			widget.NewFormItem(fmt.Sprintf("%15s", "Upper Limit:"), entryUpper),
			widget.NewFormItem(fmt.Sprintf("%15s", "Report Frequency:"), reportFreq),
		))

	progress = widget.NewProgressBar()
	progress.Resize(fyne.NewSize(200, 20))
	progress.Hide()

	infProgress = widget.NewProgressBarInfinite()
	infProgress.Hide()

	entryLayout := container.NewVBox(fixed, progress)

	calcFunc := func() {
		calcStonesMulti(entryLower.Text, entryUpper.Text, entryBase.Selected, win)
	}

	navCanvas := container.NewBorder(entryLayout, makeMultiButtons(calcFunc), nil, nil, nil)

	return navCanvas
}
func makeMultiButtons(calcFunc func()) fyne.CanvasObject {
	calcBtn = widget.NewButton("Calculate", func() {
		calcBtn.Disable()
		pauseBtn.Enable()
		stepBtn.Enable()
		stopBtn.Enable()
		go calcFunc()
	})
	pauseBtn = widget.NewButton("Pause", func() {
		pauseBtn.Disable()
		resumeBtn.Enable()
		pauseChannel <- true
	})
	stepBtn = widget.NewButton("Step", func() {
		pauseBtn.Disable()
		resumeBtn.Enable()
		stepChannel <- true
	})
	resumeBtn = widget.NewButton("Resume", func() {
		pauseBtn.Enable()
		resumeBtn.Disable()
		resumeChannel <- true
	})
	stopBtn = widget.NewButton("Stop", func() {
		stopChannel <- true
		calcBtn.Enable()
	})
	buttonLayout := container.NewGridWithColumns(2, calcBtn, pauseBtn, stepBtn, resumeBtn, stopBtn)

	pauseBtn.Disable()
	stepBtn.Disable()
	resumeBtn.Disable()
	stopBtn.Disable()
	calcBtn.Enable()

	return buttonLayout
}
func maintainHighwaterMarks(sequenceReport sequenceProgress) {

	if sequenceReport.steps > highwaterSteps {
		highwaterSteps = sequenceReport.steps
		highwaterStepsNumber = sequenceReport.number.String()
	}

	//compare n to the highwaterstone and update if n is greater
	if sequenceReport.maxStoneInt.Cmp(highmaxStone) == 1 {
		highmaxStone = new(big.Int).Set(sequenceReport.maxStoneInt)
		highwaterStone = highmaxStone.String()
		highwaterStoneNumber = sequenceReport.number.String()
	}
}
func handleSingleModeStatusReport() {

	for sequenceReport := range sequneceStatusChannel {

		stepsSlice = append(stepsSlice, float64(sequenceReport.steps))

		sf, _ := bigIntToFloat64(sequenceReport.number)
		stepsNumberSlice = append(stepsNumberSlice, sf)

		stoneStrings = sequenceReport.stonesString
		upDirectionBool = sequenceReport.upwards

		number.SetText(sequenceReport.stonesString[0])
		upDownPercentage := float64(sequenceReport.upMoves) / float64(sequenceReport.upMoves+sequenceReport.downMoves) * 100
		upDownPercentageLabel.SetText(fmt.Sprintf("%.2f%%", upDownPercentage))

		seqLen.SetText(fmt.Sprintf("%d", len(sequenceReport.stonesString)-1))
		maxStone.SetText(sequenceReport.maxStoneFloat.String())
		numUp.SetText(fmt.Sprintf("%d", sequenceReport.upMoves))
		numDown.SetText(fmt.Sprintf("%d", sequenceReport.downMoves))

		xV := make([]float64, len(sequenceReport.stonesRaw))
		for idx := 0; idx < len(sequenceReport.stonesRaw); idx++ {
			xV[idx] = float64(idx)
		}

		refreshLogChart(&xV, &sequenceReport)
		refreshAbsoluteChart(&xV, &sequenceReport)

		detailStoneList.Resize(fyne.NewSize(500, 400))
		detailStoneList.Refresh()
	}
}
func handleMultiModeStatusReport() {
	for sequenceReport := range sequneceStatusPerfChannel {

		workersFinished++
		maintainHighwaterMarks(sequenceReport)
		stepsSlice = append(stepsSlice, float64(sequenceReport.steps))
		sf, _ := bigIntToFloat64(sequenceReport.number)
		stepsNumberSlice = append(stepsNumberSlice, sf)

		if workersFinished%reportFreqencyInterval == 0 || workersFinished == workersDispatched {
			highwaterStepsLabel.SetText(fmt.Sprintf("%d", highwaterSteps))
			highwaterStepsNumberLabel.SetText(highwaterStepsNumber)
			highwaterStoneLabel.SetText(highwaterStone)
			highwaterStoneNumberLabel.SetText(highwaterStoneNumber)
			percentFinished := float64(workersFinished) / float64(workersDispatched) * 100
			progress.SetValue(percentFinished)
		}

	}
}

func calcStones(value string, base string, win fyne.Window) {

	number.SetText("")
	upDownPercentageLabel.SetText("")
	seqLen.SetText("")
	maxStone.SetText("")
	numUp.SetText("")
	numDown.SetText("")
	clearCharts()

	nv, ok := checkValidation(value, base, win)
	if !ok {
		return
	}
	rep := Collatz(nv, nil, 100)
	calcSingleBtn.Enable()

	if sequneceStatusChannel != nil {
		sequneceStatusChannel <- rep
	}
}
func calcStonesMulti(lower string, upper string, base string, win fyne.Window) {

	highwaterSteps = 0
	highwaterStepsNumber = ""
	highwaterStone = ""
	highwaterStoneNumber = ""
	highmaxStone = big.NewInt(0)
	progress.SetValue(0)
	clearCharts()
	progress.Show()

	nl, ok := checkValidation(lower, base, win)
	if !ok {
		return
	}

	nu, ok := checkValidation(upper, base, win)
	if !ok {
		return
	}

	if nu.Cmp(&nl) == -1 {
		dialog.ShowInformation("Number Format Error", "Upper limit is smaller than the lower limit", win)
		return
	}

	// Create the worker threads
	for i := 0; i < 300; i++ {
		w := &collatzWorker{}
		workersThreadSafeSlice.Push(w)
		w.Start(i)
	}

	workersDispatched = 0
	workersFinished = 0

	progress.Min = 0
	progress.Max = 100
	// Distribute the work
	for nl.Cmp(&nu) == -1 {
		wg.Add(1)
		n := new(big.Int).Set(&nl)

		workDistributorChannel <- *n
		workersDispatched++
		nl.Add(&nl, oneBig)
	}

	// Wait for the workers to finish
	wg.Wait()

	progress.Hide()
	infProgress.Show()
	// Stop the workers
	for _, worker := range workersThreadSafeSlice.workers {
		wg.Add(1)
		worker.finishedChannel <- true
	}

	wg.Wait()

	calcBtn.Enable()
	stepBtn.Disable()
	pauseBtn.Disable()
	resumeBtn.Disable()
	stopBtn.Disable()
	//	return steps

	refreshSequenceChart()
	infProgress.Hide()
}

func clearCharts() {
	stonesChart.RemoveAll()
	stonesChart.Refresh()

	stonesLogChart.RemoveAll()
	stonesLogChart.Refresh()

	sequenceLengthChart.RemoveAll()
	sequenceLengthChart.Refresh()
}
func refreshAbsoluteChart(xV *[]float64, sequenceReport *sequenceProgress) {
	graphAbsolute := chart.Chart{
		Series: []chart.Series{
			chart.ContinuousSeries{
				XValues: *xV,
				YValues: sequenceReport.stonesRaw,
			},
		},
		YAxis: chart.YAxis{
			Style:     chart.Shown(),
			NameStyle: chart.Shown(),
			Range:     &chart.ContinuousRange{},
		},
	}
	bufferAbs := bytes.NewBuffer([]byte{})
	graphAbsolute.Render(chart.PNG, bufferAbs)

	stonesChart.RemoveAll()
	stonesChart.Refresh() // ensures UI reflects the change

	absCanvas := canvas.NewImageFromReader(bufferAbs, "chart.png")
	absCanvas.SetMinSize(fyne.NewSize(200, 200))

	stonesChart.Add(absCanvas)
	stonesChart.Refresh()
}

func refreshLogChart(xV *[]float64, sequenceReport *sequenceProgress) {
	graphLog := chart.Chart{
		Series: []chart.Series{
			chart.ContinuousSeries{
				XValues: *xV,
				YValues: sequenceReport.stonesRaw,
			},
		},
		YAxis: chart.YAxis{
			Style:     chart.Shown(),
			NameStyle: chart.Shown(),
			Range:     &chart.LogarithmicRange{},
		},
	}
	bufferLog := bytes.NewBuffer([]byte{})
	graphLog.Render(chart.PNG, bufferLog)

	stonesLogChart.RemoveAll()
	stonesLogChart.Refresh() // ensures UI reflects the change

	logCanvas := canvas.NewImageFromReader(bufferLog, "chart.png")
	logCanvas.SetMinSize(fyne.NewSize(200, 200))

	stonesLogChart.Add(logCanvas)
	stonesLogChart.Refresh()
}

func refreshSequenceChart() {
	viridisByY := func(xr, yr chart.Range, index int, x, y float64) drawing.Color {
		return chart.Viridis(y, yr.GetMin(), yr.GetMax())
	}

	graphSeq := chart.Chart{
		XAxis: chart.XAxis{
			Name: "The XAxis",
			Style: chart.Style{
				Hidden: true,
			},
		},
		YAxis: chart.YAxis{
			Name: "The YAxis",
			Style: chart.Style{
				Hidden: true,
			},
			NameStyle: chart.Shown(),
			Range:     &chart.ContinuousRange{},
		},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Style: chart.Style{
					StrokeWidth:      chart.Disabled,
					DotWidth:         1,
					DotColorProvider: viridisByY,
				},
				XValues: stepsNumberSlice,
				YValues: stepsSlice,
			},
		},
	}
	bufferSeq := bytes.NewBuffer([]byte{})
	graphSeq.Render(chart.PNG, bufferSeq)

	sequenceLengthChart.RemoveAll()
	sequenceLengthChart.Refresh()

	seqCanvas := canvas.NewImageFromReader(bufferSeq, "chart.png")
	seqCanvas.SetMinSize(fyne.NewSize(400, 400))

	sequenceLengthChart.Add(seqCanvas)
	sequenceLengthChart.Refresh()
}

func bigIntToFloat64(x *big.Int) (float64, error) {
	// Convert big.Int to big.Float
	fx := new(big.Float).SetInt(x)

	// Convert big.Float to float64
	f64, acc := fx.Float64()

	if acc == big.Below { // If the conversion is below the minimum float64 value
		return 0, fmt.Errorf("big.Float value is below the minimum float64 value")
	} else if acc == big.Above { // If the conversion is above the maximum float64 value
		return 0, fmt.Errorf("big.Float value is above the maximum float64 value")
	}

	// Return the float64 value and the accuracy
	return f64, nil
}

func removeSpaces(s string) string {
	return strings.ReplaceAll(s, " ", "")
}

func checkValidation(s string, sb string, win fyne.Window) (n big.Int, ok bool) {
	if s == "" {
		return *zeroBig, false
	}
	number := new(big.Int)

	var base int
	switch sb {
	case "Base 2":
		base = 2
	case "Base 10":
		base = 10
	case "Base 16":
		base = 16
	case "Base 36":
		base = 36
	}
	_, ok = number.SetString(s, base)
	if !ok {
		dialog.ShowInformation("Number Format Error", fmt.Sprintf("The entry %s is not a valid input for Base %d", s, base), win)
		return *zeroBig, false
	}
	return *number, true
}
