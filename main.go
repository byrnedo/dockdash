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

const MaxContainers = 1000
const MaxHorizPosition = 3

var (
	Trace   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

func mapValuesSorted(mapToSort map[string]*goDocker.Container) (sorted []*goDocker.Container) {

	sorted = make([]*goDocker.Container, len(mapToSort))
	var keys []string
	for k := range mapToSort {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// To perform the opertion you want
	count := 0
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
	p.Border.Label = "Text Box"
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

		names[index-offset] = strings.TrimLeft(cont.Name, "/")
		switch infoType {
		case ImageInfo:
			info[index-offset] = strings.Replace(cont.Config.Image, "dockerregistry.pagero.local", "d.p.l", 1)
		case PortInfo:
			portStr := ""
			for intPort, extHostPortList := range cont.NetworkSettings.Ports {
				for _, extHostPort := range extHostPortList {
					portStr = intPort.Port() + "->" + extHostPort.HostIP + ":" + extHostPort.HostPort + ","
				}
			}
			info[index-offset] = portStr
		case VolumesInfo:
		case TimeInfo:
		default:
			Error.Println("Unhandled info type", infoType)
		}
	}
	return names, info
}

func updateContainerList(rightList *ui.List, leftList *ui.List, containers map[string]*goDocker.Container, offset int, horizPosition DockerInfoType) {
	names, info := getNameAndInfoOfContainers(containers, offset, horizPosition)
	rightList.Height = len(containers) + 2
	leftList.Height = len(containers) + 2
	rightList.Items = names
	leftList.Items = info
}

/*
 *func updateStatisticsRoutines(cl *dockerClient.DockerClient, event *goDocker.APIEvents, deadContainerChan chan<- string, startedContainerChan chan<- string, statsChan chan *goDocker.Stats, errChan chan<- string) {
 *    switch event.Status {
 *    case "die":
 *        go stopGettingStats(cl, event.ID, errChan)
 *    case "start":
 *        go startGettingStats(cl, event.ID, statsChan, errChan)
 *    }
 *}
 *
 *func startGettingStats(cl *dockerClient.DockerClient, id string, statsChan chan *goDocker.Stats, errChan chan<- string) {
 *    if err := cl.Stats(goDocker.StatsOptions{id, statsChan, true, nil, 0}); err != nil {
 *        errChan <- err.Error()
 *    }
 *}
 *
 *func stopGettingStats(cl *dockerClient.DockerClient, id string, errChan chan<- string) {
 *
 *}
 */

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
	errorBar := createErrorBar()
	statusBar := createStatusBar()

	containerListLeft := createContainerList()
	containerListLeft.Border.Label = "Name"
	containerListRight := createContainerList()
	containerListRight.Border.Label = "Image"

	ui.Body.AddRows(
		ui.NewRow(
			ui.NewCol(12, 0, exitBar),
		),
		ui.NewRow(
			ui.NewCol(12, 0, errorBar),
		),
		ui.NewRow(
			ui.NewCol(12, 0, statusBar),
		),
		ui.NewRow(
			ui.NewCol(3, 0, containerListLeft),
			ui.NewCol(9, 0, containerListRight),
		),
	)

	// calculate layout
	ui.Body.Align()

	drawChan := make(chan bool)
	/*
	 *errChan := make(chan string, 10)
	 */
	dockerEventChan := make(chan *goDocker.APIEvents, 10)
	containersChan := make(chan map[string]*goDocker.Container, 10)
	listStartOffsetChan := make(chan int)
	horizPositionChan := make(chan int)
	maxOffsetChan := make(chan int)
	deadContainerChan := make(chan string, 10)
	startedContainerChan := make(chan string, 10)
	doneChan := make(chan bool)
	evtChan := ui.EventCh()
	/*
	 *statsChan := make(chan *goDocker.Stats)
	 */

	err = docker.AddEventListener(dockerEventChan)
	if err != nil {
		panic("Failed to add event listener: " + err.Error())
	}

	defer func() {
		if err := docker.RemoveEventListener(dockerEventChan); err != nil {
			panic(err)
		}
	}()

	uiRoutine := func() {
		var horizPosition int = 0
		var offset int = 0
		var maxOffset int = 0
		for {
			select {
			case e := <-evtChan:
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
							horizPositionChan <- offset
						case ui.KeyArrowRight:
							if horizPosition < MaxHorizPosition {
								horizPosition++
							}
							horizPositionChan <- offset
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
			case _ = <-drawChan:
				Info.Println("Got draw event")
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
			case removedContainerID := <-deadContainerChan:
				Info.Println("Got dead container event")
				delete(currentContainers, removedContainerID)
				maxOffsetChan <- len(currentContainers) - 1
				containersChan <- currentContainers
			}
		}
	}
	go containerChangeRoutine()

	Info.Println("Spinning off update widgets routine")
	updateWidgets := func() {
		lastContainersList := make(map[string]*goDocker.Container)
		offset := 0
		horizPosition := 0
		for {
			select {
			case hp := <-horizPositionChan:
				horizPosition = hp
				Info.Println("Got changed horiz position", horizPosition)
			case containers := <-containersChan:
				Info.Println("Got containers changed event")
				updateContainerList(containerListLeft, containerListRight, containers, offset, DockerInfoType(horizPosition))
				lastContainersList = containers
				drawChan <- true
			case offset = <-listStartOffsetChan:
				Info.Println("Got list offset of", offset)
				updateContainerList(containerListLeft, containerListRight, lastContainersList, offset, DockerInfoType(horizPosition))
				drawChan <- true
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
	drawChan <- true
	Info.Println("Listing intial", len(containers), "containers as started")
	for _, cont := range containers {
		startedContainerChan <- cont.ID
	}

	<-doneChan

}
