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
	ID    string
	Stats *goDocker.Stats
}

type ContainersMsg struct {
	Left  *ui.List
	Right *ui.List
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
	drawStatsChan        = make(chan *ui.BarChart)
)

func mapValuesSorted(mapToSort map[string]*goDocker.Container) (sorted []*goDocker.Container) {

	sorted = make([]*goDocker.Container, len(mapToSort))
	var keys []string
	for k := range mapToSort {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// To perform the opertion you want
	var count = 0
	for _, k := range keys {
		sorted[count] = mapToSort[k]
		count++
	}
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

func createExitBar() (p *ui.Par) {
	p = ui.NewPar(":PRESS q TO QUIT")
	p.Height = 3
	p.TextFgColor = ui.ColorWhite
	p.Border.Label = "Dockdash"
	p.Border.FgColor = ui.ColorCyan
	return
}

func createErrorBar() (p *ui.Par) {
	p = ui.NewPar("")
	p.Height = 3
	p.TextFgColor = ui.ColorRed
	p.HasBorder = false
	return
}

func createStatusBar() (p *ui.Par) {
	p = ui.NewPar("")
	p.Height = 3
	p.TextFgColor = ui.ColorGreen
	p.HasBorder = true
	return
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
	list.ItemFgColor = ui.ColorYellow
	list.HasBorder = false
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
	return strings.TrimLeft(portsStr, ",")
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
			info[index-offset] = strings.Replace(cont.Config.Image, "dockerregistry.pagero.local", "d.p.l", 1)
		case PortInfo:
			info[index-offset] = createPortsString(cont.NetworkSettings.Ports)
		case VolumesInfo:
			volStr := ""
			for intVol, hostVol := range cont.Volumes {
				volStr += intVol + ":" + hostVol + ","
			}
			info[index-offset] = strings.TrimRight(volStr, ",")
		case TimeInfo:
			info[index-offset] = cont.Created.String()
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

func updateStatsBarChart(statsList map[string]*goDocker.Stats) *ui.BarChart {

	statsChart := ui.NewBarChart()
	statsChart.DataLabels = make([]string, len(statsList))
	statsChart.Data = make([]int, len(statsList))
	count := 0
	for key, nums := range statsList {
		statsChart.DataLabels[count] = key[:2]
		statsChart.Data[count] = int(nums.CPUStats.CPUUsage.TotalUsage / 1000000000)
		count++
	}
	return statsChart
}

func makeLayout(exitBar *ui.Par, statsChart *ui.BarChart, leftList *ui.List, rightList *ui.List) {

	ui.Body.AddRows(
		ui.NewRow(
			ui.NewCol(12, 0, exitBar),
		),
		ui.NewRow(
			ui.NewCol(12, 0, statsChart),
		),
		ui.NewRow(
			ui.NewCol(3, 0, leftList),
			ui.NewCol(9, 0, rightList),
		),
	)
}

func main() {
	logPath := "/tmp/dockdash.log"
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

	exitBar := createExitBar()

	containerListLeft := createContainerList()
	containerListLeft.Border.Label = "Name"
	containerListRight := createContainerList()
	containerListRight.Border.Label = "Image"

	statsChart := ui.NewBarChart()
	statsChart.HasBorder = false
	statsChart.Height = 10

	makeLayout(exitBar, statsChart, containerListLeft, containerListRight)

	// calculate layout
	ui.Body.Align()

	/*
	 *errChan := make(chan string, 10)
	 */

	// Statistics
	startGatheringStatisticsChan := make(chan string)
	stopGatheringStatisticsChan := make(chan string)

	statsResultsChan := make(chan *StatsResult)
	statsResultsDoneChan := make(chan string)

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
		statsList := make(map[string]*goDocker.Stats)
		for {
			select {
			case msg := <-statsResultsChan:
				statsList[msg.ID] = msg.Stats
				statsChart := updateStatsBarChart(statsList)
				drawStatsChan <- statsChart
			case id := <-statsResultsDoneChan:
				delete(statsList, id)
				statsChart := updateStatsBarChart(statsList)
				drawStatsChan <- statsChart
			}
		}
	}
	go statsRenderingRoutine()

	statsHandlingRoutine := func() {
		statsDoneChannels := make(map[string]chan bool)
		startsResultInterceptChannels := make(map[string]chan *goDocker.Stats)
		startsResultInterceptDoneChannels := make(map[string]chan bool)

		closeAndDeleteChannels := func(id string) {
			close(statsDoneChannels[id])
			delete(statsDoneChannels, id)
			delete(startsResultInterceptChannels, id)
			statsResultsDoneChan <- id
		}

		for {
			select {
			case id := <-startGatheringStatisticsChan:

				statsDoneChannels[id] = make(chan bool, 1)
				startsResultInterceptChannels[id] = make(chan *goDocker.Stats)
				startsResultInterceptDoneChannels[id] = make(chan bool)

				spinOffStatsInterceptor := func() {
					for {
						select {
						case stat := <-startsResultInterceptChannels[id]:
							statsResultsChan <- &StatsResult{id, stat}
						case _ = <-startsResultInterceptDoneChannels[id]:
							return
						}
					}
				}
				go spinOffStatsInterceptor()

				Info.Println("Starting stats routine for", id)
				spinOffStatsListener := func() {
					if err := docker.Stats(goDocker.StatsOptions{id, startsResultInterceptChannels[id], true, statsDoneChannels[id], 0}); err != nil {
						Error.Println("Error starting statistics handler for id", id, ":", err.Error())
						startsResultInterceptDoneChannels[id] <- true
					}
					closeAndDeleteChannels(id)
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
		var horizPosition int = 0
		var offset int = 0
		var maxOffset int = 0
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
			case newStatsChart := <-drawStatsChan:
				Info.Println("Got draw stats event")
				statsChart.Data = newStatsChart.Data
				statsChart.DataLabels = newStatsChart.DataLabels
				ui.Render(ui.Body)
			}
		}
	}
	go uiRoutine()

	Info.Println("Sending initial draw signal")

	//handle container addition/removal
	Info.Println("Spinning off container change routine")
	containerChangeRoutine := func() {
		currentContainers := make(map[string]*goDocker.Container)
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
				startGatheringStatisticsChan <- cont.ID

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
		var lastContainersList = make(map[string]*goDocker.Container)
		var offset = 0
		var horizPosition = 0
		var leftList = ui.NewList()
		var rightList = ui.NewList()

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
