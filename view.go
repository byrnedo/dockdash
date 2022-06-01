package main

import (
	"fmt"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

var (
	titleStyle = ui.Style{Fg: ui.ColorGreen, Bg: ui.ColorClear}
)

type uiEvent int

const (
	KeyArrowUp uiEvent = 1 << iota
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight
	KeyCtrlC
	KeyCtrlD
	KeyQ
	Resize
	KeyI
)

type dockerInfoType int

const (
	ImageInfo dockerInfoType = iota
	Names
	PortInfo
	BindInfo
	CommandInfo
	EntrypointInfo
	EnvInfo
	VolumesInfo
	TimeInfo
)

var infoHeaders = map[dockerInfoType]string{
	ImageInfo:      "Image",
	Names:          "Names",
	PortInfo:       "Ports",
	BindInfo:       "Mounts",
	CommandInfo:    "Command",
	EntrypointInfo: "Entrypoint",
	EnvInfo:        "Envs",
	VolumesInfo:    "Volumes",
	TimeInfo:       "Created At",
}

const maxContainers = 1000
const maxHorizPos = int(TimeInfo)

type view struct {
	Grid     *ui.Grid
	InfoBar  *widgets.Paragraph
	CpuChart *widgets.BarChart
	MemChart *widgets.BarChart
	NameList *widgets.List
	InfoList *widgets.List
}

func createBarChart() *widgets.BarChart {

	chart := widgets.NewBarChart()
	chart.Border = true
	chart.NumFormatter = func(f float64) string {
		return fmt.Sprintf("%02.0f", f)
	}
	//chart.MaxVal = 100
	chart.TitleStyle = titleStyle
	return chart
}

func createContainerList() *widgets.List {
	list := widgets.NewList()
	list.TitleStyle = titleStyle
	list.TextStyle = ui.Style{Fg: ui.ColorCyan, Bg: ui.ColorClear}
	list.SelectedRowStyle = ui.Style{Fg: ui.ColorCyan, Bg: ui.ColorClear}
	list.Border = true
	return list
}

func NewView() *view {

	var view = view{}

	view.InfoBar = widgets.NewParagraph()
	view.InfoBar.Border = false
	view.InfoBar.Text = ""
	view.InfoBar.TitleStyle = titleStyle

	view.NameList = createContainerList()
	view.NameList.Title = "Name"

	view.InfoList = createContainerList()
	view.InfoList.Title = "Image"

	view.CpuChart = createBarChart()
	view.CpuChart.Title = "%CPU"

	view.MemChart = createBarChart()
	view.MemChart.Title = "%MEM"

	return &view
}

func (v *view) SetLayout() {
	v.Grid = ui.NewGrid()
	v.ResetSize()
	v.Grid.Set(
		ui.NewRow(1.0/12,
			ui.NewCol(1.0, v.InfoBar),
		),
		ui.NewRow(3.0/12,
			ui.NewCol(1.0, v.CpuChart),
		),
		ui.NewRow(3.0/12,
			ui.NewCol(1.0, v.MemChart),
		),
		ui.NewRow(5.0/12,
			ui.NewCol(4.0/12, v.NameList),
			ui.NewCol(8.0/12, v.InfoList),
		),
	)
}

func (v *view) ResetSize() {
	termWidth, termHeight := ui.TerminalDimensions()
	if termWidth > 20 {
		v.Grid.SetRect(0, 0, termWidth, termHeight)
	}
}

func (v *view) Render() {
	//ui.Clear()
	ui.Render(v.Grid)
}

func (v *view) UpdateStats(statsCharts *StatsMsg, offset int) {

	statsCharts.CpuChart.Offset(offset).UpdateBarChart(v.CpuChart)
	statsCharts.MemChart.Offset(offset).UpdateBarChart(v.MemChart)

	v.Render()
}

func (v *view) RenderContainers(containers containerMap, infoType dockerInfoType, listOffset int, inspectMode bool) {
	names, info := containers.namesAndInfo(listOffset, infoType, inspectMode)
	v.NameList.Rows = names
	v.InfoList.Rows = info
	v.InfoList.Title = infoHeaders[infoType]
	v.Render()
}

func (v view) UpdateInfoBar(currentContainers containerMap, currentStats *StatsMsg) {
	var (
		numCons  = len(currentContainers)
		totalCpu = 0.0
		totalMem = 0.0
	)
	if currentStats != nil {
		totalCpu = sum(currentStats.CpuChart.Data...)
		totalMem = sum(currentStats.MemChart.Data...)
	}

	v.InfoBar.Text = fmt.Sprintf(" Cons:%d  Total CPU:%d%%  Total Mem:%d%%", numCons, int(totalCpu), int(totalMem))
	v.InfoBar.Title = "Dockdash"
	v.Render()
}

func sum(nums ...float64) float64 {
	total := 0.0
	for _, num := range nums {
		total += num
	}
	return total
}
