package main

import (
	"github.com/byrnedo/dockdash/dockerClient"
	goDocker "github.com/fsouza/go-dockerclient"
	ui "github.com/gizak/termui"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

var (
	Trace   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

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

func getNamesAndImagesOfContainers(containers map[string]*goDocker.Container) ([]string, []string) {
	names := make([]string, len(containers))
	images := make([]string, len(containers))
	var count int
	for _, cont := range containers {
		names[count] = cont.Name
		images[count] = strings.Replace(cont.Image, "dockerregistry.pagero.local", "d.p.l", 1)
		count++
	}
	return names, images
}

func updateContainerList(listOfNames *ui.List, listOfImages *ui.List, containers map[string]*goDocker.Container) {
	names, images := getNamesAndImagesOfContainers(containers)
	listOfNames.Height = len(containers) + 2
	listOfImages.Height = len(containers) + 2
	listOfNames.Items = names
	listOfImages.Items = images
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

	containerListOfNames := createContainerList()
	containerListOfNames.Border.Label = "Name"
	containerListOfImages := createContainerList()
	containerListOfImages.Border.Label = "Image"

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
			ui.NewCol(3, 0, containerListOfNames),
			ui.NewCol(9, 0, containerListOfImages),
		),
	)

	// calculate layout
	ui.Body.Align()

	drawChan := make(chan bool)
	/*
	 *errChan := make(chan string, 10)
	 */
	eventsChan := make(chan *goDocker.APIEvents, 10)
	containersChan := make(chan map[string]*goDocker.Container, 10)
	deadContainerChan := make(chan string, 10)
	startedContainerChan := make(chan string, 10)
	doneChan := make(chan bool)
	evtChan := ui.EventCh()
	/*
	 *statsChan := make(chan *goDocker.Stats)
	 */

	err = docker.AddEventListener(eventsChan)
	if err != nil {
		panic("Failed to add event listener: " + err.Error())
	}

	defer func() {
		if err := docker.RemoveEventListener(eventsChan); err != nil {
			panic(err)
		}
	}()

	uiRoutine := func() {
		for {
			select {
			case e := <-evtChan:
				Info.Println("Got ui event:", e)
				if e.Type == ui.EventKey && e.Ch == 'q' {
					doneChan <- true
				}
				if e.Type == ui.EventResize {
					ui.Body.Width = ui.TermWidth()
					ui.Body.Align()
				}
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
				containersChan <- currentContainers
			case removedContainerID := <-deadContainerChan:
				Info.Println("Got dead container event")
				delete(currentContainers, removedContainerID)
				containersChan <- currentContainers
			}
		}
	}
	go containerChangeRoutine()

	Info.Println("Spinning off update widgets routine")
	updateWidgets := func() {
		for {
			select {
			case containers := <-containersChan:
				Info.Println("Got containers changed event")
				updateContainerList(containerListOfNames, containerListOfImages, containers)
				drawChan <- true
			}
		}
	}
	go updateWidgets()

	//setup initial containers
	containers, _ := docker.ListContainers(goDocker.ListContainersOptions{})
	drawChan <- true
	Info.Println("Listing intial", len(containers), "containers as started")
	for _, cont := range containers {
		startedContainerChan <- cont.ID
	}

	<-doneChan

}
