package main

import (
	"github.com/byrnedo/dockdash/dockerClient"
	goDocker "github.com/fsouza/go-dockerclient"
	ui "github.com/gizak/termui"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"
	"time"
)

type DockerInfoType int

const (
	ImageInfo DockerInfoType = iota
	PortInfo
	VolumesInfo
	TimeInfo
)

var InfoHeaders map[DockerInfoType]string = map[DockerInfoType]string{
	ImageInfo:   "Images",
	PortInfo:    "Ports",
	VolumesInfo: "Volumes",
	TimeInfo:    "Created At",
}

const MaxContainers = 1000
const MaxHorizPosition = int(TimeInfo)

type StatsResult struct {
	Container *goDocker.Container
	Stats     *goDocker.Stats
}

type StatsResultSlice []*StatsResult

func (p StatsResultSlice) Len() int {
	return len(p)
}

func (p StatsResultSlice) Less(i, j int) bool {
	return p[i].Container.State.StartedAt.Before(p[j].Container.State.StartedAt)
}

func (p StatsResultSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

type ContainersMsg struct {
	Left  *ui.List
	Right *ui.List
}

type ContainerSlice []*goDocker.Container

func (p ContainerSlice) Len() int {
	return len(p)
}

func (p ContainerSlice) Less(i, j int) bool {
	return p[i].State.StartedAt.Before(p[j].State.StartedAt)
}

func (p ContainerSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

type StatsCharts struct {
	CpuChart *ui.BarChart
	MemChart *ui.BarChart
}

var (
	Trace   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger

	dockerEventChan      = make(chan *goDocker.APIEvents, 10)
	containersChan       = make(chan map[string]*goDocker.Container)
	listStartOffsetChan  = make(chan int)
	horizPositionChan    = make(chan int)
	maxOffsetChan        = make(chan int)
	deadContainerChan    = make(chan string)
	startedContainerChan = make(chan string)
	doneChan             = make(chan bool)
	uiEventChan          = ui.EventCh()
	drawContainersChan   = make(chan *ContainersMsg)
	drawStatsChan        = make(chan *StatsCharts)

	startGatheringStatisticsChan = make(chan *goDocker.Container)
	stopGatheringStatisticsChan  = make(chan string)

	statsResultsChan     = make(chan *StatsResult)
	statsResultsDoneChan = make(chan string)
)

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

func InitLog(
	traceHandle io.Writer,
	infoHandle io.Writer,
	warningHandle io.Writer,
	errorHandle io.Writer) {

	Trace = log.New(traceHandle,
		"TRACE: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Info = log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Warning = log.New(warningHandle,
		"WARNING: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Error = log.New(errorHandle,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}

func createCPUGauge() *ui.Gauge {
	cpuGauge := ui.NewGauge()
	cpuGauge.Percent = 50
	cpuGauge.Height = 3
	cpuGauge.Border.Label = "CPU Usage"
	cpuGauge.BarColor = ui.ColorRed
	cpuGauge.Border.FgColor = ui.ColorWhite
	cpuGauge.Border.LabelFgColor = ui.ColorCyan
	return cpuGauge
}

func createMemGauge() *ui.Gauge {
	memGauge := ui.NewGauge()
	memGauge.Percent = 50
	memGauge.Height = 3
	memGauge.Border.Label = "Mem. Usage"
	memGauge.BarColor = ui.ColorRed
	memGauge.Border.FgColor = ui.ColorWhite
	memGauge.Border.LabelFgColor = ui.ColorCyan
	return memGauge
}

func createContainerList() *ui.List {
	list := ui.NewList()
	list.ItemFgColor = ui.ColorCyan
	list.HasBorder = true
	return list
}

func createDockerLineChart() *ui.LineChart {
	lc := ui.NewLineChart()
	lc.Border.Label = "Container Numbers"
	//lc.Data = sinps
	lc.Height = 10
	lc.AxesColor = ui.ColorWhite
	lc.LineColor = ui.ColorRed | ui.AttrBold
	lc.Mode = "line"
	return lc
}

func createPortsString(ports map[goDocker.Port][]goDocker.PortBinding) (portsStr string) {

	for intPort, extHostPortList := range ports {
		if len(extHostPortList) == 0 {
			portsStr += intPort.Port() + "->N/A,"
		}
		for _, extHostPort := range extHostPortList {
			portsStr += intPort.Port() + "->" + extHostPort.HostIP + ":" + extHostPort.HostPort + ","
		}
	}
	return strings.TrimRight(portsStr, ",")
}

func getNameAndInfoOfContainers(containers map[string]*goDocker.Container, offset int, infoType DockerInfoType) ([]string, []string) {
	if offset > len(containers) {
		offset = len(containers) - 1
	}

	numContainersSubset := len(containers) - offset

	names := make([]string, numContainersSubset)
	info := make([]string, numContainersSubset)

	containersSorted := mapValuesSorted(containers)
	for index, cont := range containersSorted {
		if index < offset {
			continue
		}

		names[index-offset] = cont.ID[:12] + " " + strings.TrimLeft(cont.Name, "/")
		switch infoType {
		case ImageInfo:
			info[index-offset] = cont.Config.Image
		case PortInfo:
			info[index-offset] = createPortsString(cont.NetworkSettings.Ports)
		case VolumesInfo:
			volStr := ""
			for intVol, hostVol := range cont.Volumes {
				volStr += intVol + ":" + hostVol + ","
			}
			info[index-offset] = strings.TrimRight(volStr, ",")
		case TimeInfo:
			info[index-offset] = cont.State.StartedAt.Format(time.RubyDate)
		default:
			Error.Println("Unhandled info type", infoType)
		}
	}
	return names, info
}

func updateContainerList(leftList *ui.List, rightList *ui.List, containers map[string]*goDocker.Container, offset int, horizPosition DockerInfoType) {
	names, info := getNameAndInfoOfContainers(containers, offset, horizPosition)
	leftList.Height = len(containers) + 2
	rightList.Height = len(containers) + 2
	leftList.Items = names
	rightList.Border.Label = InfoHeaders[horizPosition]
	rightList.Items = info
}

func updateStatsBarCharts(statsList map[string]*StatsResult) (statsCpuChart *ui.BarChart, statsMemChart *ui.BarChart) {
	// need to make new ones as otherwise we may lose some pointers if something else happens at the same time
	statsCpuChart = ui.NewBarChart()
	statsMemChart = ui.NewBarChart()

	var statsListLen = len(statsList)

	var (
		orderedList = make(StatsResultSlice, statsListLen)
	)

	statsCpuChart.DataLabels = make([]string, statsListLen)
	statsCpuChart.Data = make([]int, statsListLen)

	statsMemChart.DataLabels = make([]string, statsListLen)
	statsMemChart.Data = make([]int, statsListLen)

	count := 0
	for _, nums := range statsList {
		orderedList[count] = nums
		count++
	}

	sort.Sort(orderedList)

	for count, stats := range orderedList {
		statsCpuChart.DataLabels[count] = stats.Container.ID[:2]
		statsCpuChart.Data[count] = int(calculateCPUPercent(stats.Stats))

		statsMemChart.DataLabels[count] = stats.Container.ID[:2]
		if stats.Stats.MemoryStats.Limit != 0 {
			statsMemChart.Data[count] = int(float64(stats.Stats.MemoryStats.Usage) / float64(stats.Stats.MemoryStats.Limit) * 100)
		} else {
			statsMemChart.Data[count] = 0
		}
	}
	return statsCpuChart, statsMemChart
}

func calculateCPUPercent(v *goDocker.Stats) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
		// calculate the change for the entire system between readings
		systemDelta = float64(v.CPUStats.SystemCPUUsage - v.PreCPUStats.SystemCPUUsage)
	)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return cpuPercent
}

func makeLayout(statsCpuChart *ui.BarChart, statsMemChart *ui.BarChart, leftList *ui.List, rightList *ui.List) {

	ui.Body.AddRows(
		ui.NewRow(
			ui.NewCol(12, 0, statsCpuChart),
		),
		ui.NewRow(
			ui.NewCol(12, 0, statsMemChart),
		),
		ui.NewRow(
			ui.NewCol(3, 0, leftList),
			ui.NewCol(9, 0, rightList),
		),
	)
}

func main() {
	var logPath = "/tmp/dockdash.log"
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic("Failed to open log file " + logPath + ":" + err.Error())
	}

	InitLog(ioutil.Discard, file, file, file)

	docker, err := dockerClient.NewDockerClient()
	if err != nil {
		panic(err)
	}

	err = ui.Init()
	if err != nil {
		panic(err)
	}

	defer ui.Close()

	containerListLeft := createContainerList()
	containerListLeft.Border.Label = "Name"
	containerListRight := createContainerList()
	containerListRight.Border.Label = "Image"

	statsCpuChart := ui.NewBarChart()
	statsCpuChart.HasBorder = true
	statsCpuChart.Border.Label = "%CPU"
	statsCpuChart.Height = 10

	statsMemChart := ui.NewBarChart()
	statsMemChart.HasBorder = true
	statsMemChart.Border.Label = "%MEM"
	statsMemChart.Height = 10

	makeLayout(statsCpuChart, statsMemChart, containerListLeft, containerListRight)

	// calculate layout
	ui.Body.Align()

	// Statistics

	err = docker.AddEventListener(dockerEventChan)
	if err != nil {
		panic("Failed to add event listener: " + err.Error())
	}

	defer func() {
		if err := docker.RemoveEventListener(dockerEventChan); err != nil {
			panic(err)
		}
	}()

	statsRenderingRoutine := func() {

		var (
			statsList = make(map[string]*StatsResult)
		)
		for {
			select {
			case msg := <-statsResultsChan:
				statsList[msg.Container.ID] = msg
				statsCpuChart, statsMemChart := updateStatsBarCharts(statsList)
				drawStatsChan <- &StatsCharts{statsCpuChart, statsMemChart}
			case id := <-statsResultsDoneChan:
				delete(statsList, id)
				statsCpuChart, statsMemChart := updateStatsBarCharts(statsList)
				drawStatsChan <- &StatsCharts{statsCpuChart, statsMemChart}
			}
		}
	}
	go statsRenderingRoutine()

	statsHandlingRoutine := func() {
		var (
			statsDoneChannels                 = make(map[string]chan bool)
			startsResultInterceptChannels     = make(map[string]chan *goDocker.Stats)
			startsResultInterceptDoneChannels = make(map[string]chan bool)
		)

		closeAndDeleteChannels := func(id string) {
			close(statsDoneChannels[id])
			delete(statsDoneChannels, id)
			delete(startsResultInterceptChannels, id)
			delete(startsResultInterceptDoneChannels, id)
			statsResultsDoneChan <- id
		}

		for {
			select {
			case cont := <-startGatheringStatisticsChan:

				statsDoneChannels[cont.ID] = make(chan bool, 1)
				startsResultInterceptChannels[cont.ID] = make(chan *goDocker.Stats)
				startsResultInterceptDoneChannels[cont.ID] = make(chan bool)

				spinOffStatsInterceptor := func() {
					for {
						select {
						case stat := <-startsResultInterceptChannels[cont.ID]:
							statsResultsChan <- &StatsResult{cont, stat}
						case _ = <-startsResultInterceptDoneChannels[cont.ID]:
							return
						}
					}
				}
				go spinOffStatsInterceptor()

				Info.Println("Starting stats routine for", cont.ID)
				spinOffStatsListener := func() {
					if err := docker.Stats(goDocker.StatsOptions{cont.ID, startsResultInterceptChannels[cont.ID], true, statsDoneChannels[cont.ID], 0}); err != nil {
						Error.Println("Error starting statistics handler for id", cont.ID, ":", err.Error())
						startsResultInterceptDoneChannels[cont.ID] <- true
					}
					closeAndDeleteChannels(cont.ID)
				}
				go spinOffStatsListener()
			case id := <-stopGatheringStatisticsChan:
				Info.Println("Stopping stats routine for", id)
				statsDoneChannels[id] <- true
				startsResultInterceptDoneChannels[id] <- true
				closeAndDeleteChannels(id)
			}
		}
	}
	go statsHandlingRoutine()

	uiRoutine := func() {
		var (
			horizPosition   int       = 0
			offset          int       = 0
			maxOffset       int       = 0
			lastStatsRender time.Time = time.Time{}
		)
		for {
			select {
			case e := <-uiEventChan:
				Info.Println("Got ui event:", e)
				if e.Type == ui.EventKey {
					switch e.Ch {
					case 'q':
						doneChan <- true
					case 0:
						switch e.Key {
						case ui.KeyArrowLeft:
							if horizPosition > 0 {
								horizPosition--
							}
							horizPositionChan <- horizPosition
						case ui.KeyArrowRight:
							if horizPosition < MaxHorizPosition {
								horizPosition++
							}
							horizPositionChan <- horizPosition
						case ui.KeyArrowDown:
							if offset < maxOffset && offset < MaxContainers {
								offset++
							}
							listStartOffsetChan <- offset
							//shift the list down
						case ui.KeyArrowUp:
							if offset > 0 {
								offset--
							}
							listStartOffsetChan <- offset
							//shift the list up
						default:
							Info.Printf("Got unhandled key %d\n", e.Key)
						}
					}
				}
				if e.Type == ui.EventResize {
					ui.Body.Width = ui.TermWidth()
					ui.Body.Align()
				}
				ui.Render(ui.Body)
			case max := <-maxOffsetChan:
				maxOffset = max
			case cons := <-drawContainersChan:
				Info.Println("Got draw containers event")
				containerListLeft.Height = cons.Left.Height
				containerListLeft.Items = cons.Left.Items
				containerListRight.Height = cons.Right.Height
				containerListRight.Items = cons.Right.Items
				containerListRight.Border.Label = cons.Right.Border.Label
				ui.Render(ui.Body)

			case newStatsCharts := <-drawStatsChan:
				if time.Now().Sub(lastStatsRender) > 500*time.Millisecond {
					Info.Println("Got draw stats event")
					statsCpuChart.Data = newStatsCharts.CpuChart.Data[offset:]
					statsCpuChart.DataLabels = newStatsCharts.CpuChart.DataLabels[offset:]
					statsMemChart.Data = newStatsCharts.MemChart.Data[offset:]
					statsMemChart.DataLabels = newStatsCharts.MemChart.DataLabels[offset:]
					ui.Render(ui.Body)
					lastStatsRender = time.Now()
				}
			}
		}
	}
	go uiRoutine()

	//handle container addition/removal
	Info.Println("Spinning off container change routine")
	containerChangeRoutine := func() {
		var currentContainers = make(map[string]*goDocker.Container)
		for {
			select {
			case newContainerID := <-startedContainerChan:
				Info.Println("Got new container event")
				cont, err := docker.InspectContainer(newContainerID)
				if err != nil {
					Error.Println("Failed to inspect new container", newContainerID, ":", err)
					continue
				}
				currentContainers[cont.ID] = cont
				maxOffsetChan <- len(currentContainers) - 1
				containersChan <- currentContainers
				startGatheringStatisticsChan <- cont

			case removedContainerID := <-deadContainerChan:
				Info.Println("Got dead container event")
				delete(currentContainers, removedContainerID)
				maxOffsetChan <- len(currentContainers) - 1
				containersChan <- currentContainers
				stopGatheringStatisticsChan <- removedContainerID
			}
		}
	}
	go containerChangeRoutine()

	Info.Println("Spinning off update widgets routine")
	updateWidgets := func() {
		var (
			lastContainersList = make(map[string]*goDocker.Container)
			offset             = 0
			horizPosition      = 0
			leftList           = ui.NewList()
			rightList          = ui.NewList()
		)
		updateContainersAndNotify := func() {
			updateContainerList(leftList, rightList, lastContainersList, offset, DockerInfoType(horizPosition))
			drawContainersChan <- &ContainersMsg{leftList, rightList}
		}

		for {
			select {
			case hp := <-horizPositionChan:
				horizPosition = hp
				Info.Println("Got changed horiz position", horizPosition)
				updateContainersAndNotify()
			case containers := <-containersChan:
				Info.Println("Got containers changed event")
				lastContainersList = containers
				updateContainersAndNotify()
			case offset = <-listStartOffsetChan:
				Info.Println("Got list offset of", offset)
				updateContainersAndNotify()
			}
		}
	}
	go updateWidgets()

	dockerEventRouting := func() {
		for {
			select {
			case e := <-dockerEventChan:
				switch e.Status {
				case "start":
					startedContainerChan <- e.ID
				case "die":
					deadContainerChan <- e.ID
				}
			}
		}
	}

	go dockerEventRouting()

	//setup initial containers
	containers, _ := docker.ListContainers(goDocker.ListContainersOptions{})
	ui.Render(ui.Body)
	Info.Println("Listing intial", len(containers), "containers as started")
	for _, cont := range containers {
		startedContainerChan <- cont.ID
	}

	<-doneChan

}
