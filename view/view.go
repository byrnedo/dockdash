package view

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	docklistener "github.com/byrnedo/dockdash/docklistener"
	. "github.com/byrnedo/dockdash/logger"
	goDocker "github.com/fsouza/go-dockerclient"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

var (
	titleStyle = ui.Style{Fg: ui.ColorGreen, Bg: ui.ColorClear}
)

type UIEvent int

const (
	KeyArrowUp UIEvent = 1 << iota
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight
	KeyCtrlC
	KeyCtrlD
	KeyQ
	Resize
	KeyI
)

type DockerInfoType int

const (
	ImageInfo DockerInfoType = iota
	Names
	PortInfo
	BindInfo
	CommandInfo
	EntrypointInfo
	EnvInfo
	VolumesInfo
	TimeInfo
)

var InfoHeaders map[DockerInfoType]string = map[DockerInfoType]string{
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

const MaxContainers = 1000
const MaxHorizPosition = int(TimeInfo)

type View struct {
	Grid     *ui.Grid
	Header   *widgets.Paragraph
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
	chart.MaxVal = 100
	chart.TitleStyle = titleStyle
	return chart
}

func createContainerList() *widgets.List {
	list := widgets.NewList()
	list.TitleStyle = titleStyle
	list.TextStyle = ui.Style{Fg: ui.ColorBlue, Bg: ui.ColorClear}
	list.SelectedRowStyle = ui.Style{Fg: ui.ColorBlue, Bg: ui.ColorClear}
	//list.SelectedRowStyle = ui.Style{
	//	Fg: ui.ColorCyan,
	//}
	//list.BorderStyle = ui.Style{
	//	Fg: ui.ColorBlack,
	//}
	list.Border = true
	return list
}

type ContainerSlice []*goDocker.Container

func (p ContainerSlice) Len() int {
	return len(p)
}

func (p ContainerSlice) Less(i, j int) bool {
	return p[i].State.StartedAt.After(p[j].State.StartedAt)
}

func (p ContainerSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func createDockerLineChart() *widgets.Plot {
	lc := widgets.NewPlot()
	lc.Title = "Container Numbers"
	lc.AxesColor = ui.ColorWhite
	lc.LineColors = append(lc.LineColors, ui.ColorRed)
	return lc
}

func NewView() *View {

	var view = View{}

	view.Header = widgets.NewParagraph()
	view.Header.Border = false
	view.Header.Text = " Dockdash"
	view.Header.TextStyle = titleStyle
	//view.Header.Max = 2

	view.InfoBar = widgets.NewParagraph()
	view.InfoBar.Border = false
	view.InfoBar.Text = ""
	//view.InfoBar.Height = 2

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

func (v *View) SetLayout() {
	v.Grid = ui.NewGrid()
	v.ResetSize()
	v.Grid.Set(
		ui.NewRow(1.0/8,
			ui.NewCol(1.0/2, v.Header),
		),
		ui.NewRow(1.0/8,
			ui.NewCol(1.0/2, v.InfoBar),
		),
		ui.NewRow(1.0/4,
			ui.NewCol(1.0, v.CpuChart),
		),
		ui.NewRow(1.0/4,
			ui.NewCol(1.0, v.MemChart),
		),
		ui.NewRow(1.0/4,
			ui.NewCol(4.0/12, v.NameList),
			ui.NewCol(8.0/12, v.InfoList),
		),
	)
}

func (v *View) ResetSize() {
	termWidth, termHeight := ui.TerminalDimensions()
	if termWidth > 20 {
		v.Grid.SetRect(0, 0, termWidth, termHeight)
	}
}

func (v *View) Render() {
	ui.Clear()
	ui.Render(v.Grid)
}

func applyBarChartValues(chart *widgets.BarChart, vals []float64, labels []string) {
	chart.Data = vals
	numBars := len(chart.Data)
	chart.BarColors = make([]ui.Color, numBars)
	chart.LabelStyles = make([]ui.Style, numBars)
	chart.NumStyles = make([]ui.Style, numBars)
	for i, _ := range chart.BarColors {
		chart.BarColors[i] = ui.ColorWhite
		chart.LabelStyles[i] = ui.Style{Fg: ui.ColorWhite, Bg: ui.ColorClear}
		chart.NumStyles[i] = ui.Style{Fg: ui.ColorBlack}
	}
	chart.Labels = labels
}

func (v *View) UpdateStats(statsCharts *docklistener.StatsMsg, offset int) {
	Info.Println(statsCharts)

	applyBarChartValues(v.CpuChart, statsCharts.CpuChart.Data[offset:], statsCharts.CpuChart.DataLabels[offset:])
	applyBarChartValues(v.MemChart, statsCharts.MemChart.Data[offset:], statsCharts.MemChart.DataLabels[offset:])

	v.Render()
}

func (v *View) RenderContainers(containers map[string]*goDocker.Container, infoType DockerInfoType, listOffset int, inspectMode bool) {
	names, info := getNameAndInfoOfContainers(containers, listOffset, infoType, inspectMode)
	//v.NameList.Height = len(names) + 2
	v.NameList.Rows = names
	//v.InfoList.Height = len(info) + 2
	v.InfoList.Rows = info
	v.InfoList.Title = InfoHeaders[infoType]
	v.Render()
}

func getNameAndInfoOfContainers(containers map[string]*goDocker.Container, offset int, infoType DockerInfoType, inspectMode bool) ([]string, []string) {
	var numContainers = len(containers)
	if offset > numContainers {
		offset = numContainers - 1
	}

	var (
		info                []string
		numContainersSubset = numContainers - offset
		names               = make([]string, numContainersSubset)
		containersSorted    = mapValuesSorted(containers)
		nameStr             = ""
		containerNumber     = 0
	)

	if !inspectMode {
		info = make([]string, numContainersSubset)
	}

	for index, cont := range containersSorted {
		if index < offset {
			continue
		}

		containerNumber = numContainers - index
		nameStr = strconv.Itoa(containerNumber) + ". " + cont.ID[:12] + " " + strings.TrimLeft(cont.Name, "/")

		if inspectMode && index == offset {
			names[index-offset] = "*" + nameStr
			info = createInspectModeData(index, offset, infoType, cont)
		} else {
			names[index-offset] = " " + nameStr
			if !inspectMode {
				info[index-offset] = createRegularModeData(index, offset, infoType, cont)
			}
		}

	}
	return names, info
}

func createInspectModeData(index int, offset int, infoType DockerInfoType, cont *goDocker.Container) (info []string) {
	switch infoType {
	case ImageInfo:
		info = []string{cont.Config.Image}
	case Names:
		if cont.Node != nil {
			info = []string{cont.Node.Name, cont.Name}
		} else {
			info = []string{cont.Name}
		}
	case PortInfo:
		info = createPortsSlice(cont.NetworkSettings.Ports)
	case BindInfo:
		info = make([]string, len(cont.HostConfig.Binds))
		for i, binding := range cont.HostConfig.Binds {
			info[i] = binding
		}
	case CommandInfo:
		info = make([]string, len(cont.Args))
		for i, arg := range cont.Args {
			info[i] = arg
		}
	case EnvInfo:
		info = make([]string, len(cont.Config.Env))
		for i, env := range cont.Config.Env {
			info[i] = env
		}
	case EntrypointInfo:
		info = make([]string, len(cont.Config.Entrypoint))
		for i, entrypoint := range cont.Config.Entrypoint {
			info[i] = entrypoint
		}
	case VolumesInfo:
		info = make([]string, len(cont.Volumes))
		i := 0
		for intVol, hostVol := range cont.Volumes {
			info[i] = intVol + ":" + hostVol + ""
			i++
		}
	case TimeInfo:
		info = []string{cont.State.StartedAt.Format(time.RubyDate)}
	default:
		Error.Println("Unhandled info type", infoType)
	}
	return
}

func createRegularModeData(index int, offset int, infoType DockerInfoType, cont *goDocker.Container) (info string) {

	switch infoType {
	case ImageInfo:
		info = cont.Config.Image
	case Names:
		info = cont.Name
		if cont.Node != nil {
			info = cont.Node.Name + info
		}
	case PortInfo:
		info = createPortsString(cont.NetworkSettings.Ports, ",")
	case BindInfo:
		info = strings.TrimRight(strings.Join(cont.HostConfig.Binds, ","), ",")
	case CommandInfo:
		info = cont.Path + " " + strings.Join(cont.Args, " ")
	case EnvInfo:
		info = strings.TrimRight(strings.Join(cont.Config.Env, ","), ",")
	case EntrypointInfo:
		info = strings.Join(cont.Config.Entrypoint, " ")
	case VolumesInfo:
		volStr := ""
		for intVol, hostVol := range cont.Volumes {
			volStr += intVol + ":" + hostVol + ","
		}
		info = strings.TrimRight(volStr, ",")
	case TimeInfo:
		info = cont.State.StartedAt.Format(time.RubyDate)
	default:
		Error.Println("Unhandled info type", infoType)
	}
	return
}

func mapValuesSorted(mapToSort map[string]*goDocker.Container) (sorted ContainerSlice) {

	sorted = make(ContainerSlice, len(mapToSort))
	var i = 0
	for _, val := range mapToSort {
		sorted[i] = val
		i++
	}
	sort.Sort(sorted)
	return
}

func createPortsString(ports map[goDocker.Port][]goDocker.PortBinding, sep string) (portsStr string) {

	for intPort, extHostPortList := range ports {
		if len(extHostPortList) == 0 {
			portsStr += intPort.Port() + "->N/A" + sep
		}
		for _, extHostPort := range extHostPortList {
			portsStr += intPort.Port() + "->" + extHostPort.HostIP + ":" + extHostPort.HostPort + sep
		}
	}
	return strings.TrimRight(portsStr, sep)
}

func createPortsSlice(ports map[goDocker.Port][]goDocker.PortBinding) (portsSlice []string) {

	portsSlice = make([]string, len(ports))
	i := 0
	for intPort, extHostPortList := range ports {
		if len(extHostPortList) == 0 {
			portsSlice[i] = intPort.Port() + "->N/A"
		}
		for _, extHostPort := range extHostPortList {
			portsSlice[i] = intPort.Port() + "->" + extHostPort.HostIP + ":" + extHostPort.HostPort
		}
		i++
	}
	return
}
