package main

import (
	"fmt"
	"github.com/byrnedo/dockdash/dockerClient"
	goDocker "github.com/fsouza/go-dockerclient"
	ui "github.com/gizak/termui"
	"strings"
	"time"
)

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

func getNamesAndImagesOfRunning(cl *dockerClient.DockerClient, containerChan chan<- []goDocker.APIContainers) ([]string, []string) {
	containers, _ := cl.ListContainers(goDocker.ListContainersOptions{})
	names := make([]string, len(containers))
	images := make([]string, len(containers))
	for i, cont := range containers {
		names[i] = strings.TrimLeft(strings.Join(cont.Names, ""), "/")
		images[i] = strings.Replace(cont.Image, "dockerregistry.pagero.local", "d.p.l", 1)
	}
	containerChan <- containers
	return names, images
}

func updateStatisticsRoutines(cl *dockerClient.DockerClient, event *goDocker.APIEvents, deadContainerChan chan<- string, startedContainerChan chan<- string, statsChan chan *goDocker.Stats, errChan chan<- string) {
	switch event.Status {
	case "die":
		go stopGettingStats(cl, event.ID, errChan)
	case "start":
		go startGettingStats(cl, event.ID, statsChan, errChan)
	}
}

func startGettingStats(cl *dockerClient.DockerClient, id string, statsChan chan *goDocker.Stats, errChan chan<- string) {
	if err := cl.Stats(goDocker.StatsOptions{id, statsChan, true, nil, 0}); err != nil {
		errChan <- err.Error()
	}
}

func stopGettingStats(cl *dockerClient.DockerClient, id string, errChan chan<- string) {

}

func main() {

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

	errChan := make(chan string, 10)
	eventsChan := make(chan *goDocker.APIEvents, 10)
	containersChan := make(chan []goDocker.APIContainers, 10)
	deadContainerChan := make(chan string, 10)
	startedContainerChan := make(chan string, 10)
	statsChan := make(chan *goDocker.Stats)

	defer func() {
		if err := docker.RemoveEventListener(eventsChan); err != nil {
			panic(err)
		}
	}()

	err = docker.AddEventListener(eventsChan)
	if err != nil {
		panic("Failed to add event listener: " + err.Error())
	}

	drawContainerList := func(t int) {

		names, images := getNamesAndImagesOfRunning(docker, containersChan)
		containerListOfNames.Items = names
		containerListOfImages.Items = images
		containerListOfNames.Height = len(names) + 1
		containerListOfImages.Height = len(images) + 1

		//lc.Data = append(lc.Data, float64(len(names)))
		ui.Render(ui.Body)

	}
	drawContainerList(0)

	containers, _ := docker.ListContainers(goDocker.ListContainersOptions{})
	for _, cont := range containers {
		startedContainerChan <- cont.ID
	}

	evt := ui.EventCh()

	i := 0
	for {
		select {
		case stats := <-statsChan:
			statusBar.Text = "Got stats"
			errChan <- fmt.Sprintf("%f", stats.CPUStats.SystemCPUUsage)
		case e := <-evt:
			statusBar.Text = "Got ui event"
			if e.Type == ui.EventKey && e.Ch == 'q' {
				return
			}
			if e.Type == ui.EventResize {
				ui.Body.Width = ui.TermWidth()
				ui.Body.Align()
			}
		case err := <-errChan:
			statusBar.Text = "Got error"
			errorBar.Text = err
		case e := <-eventsChan:
			statusBar.Text = "Got docker event"
			updateStatisticsRoutines(docker, e, deadContainerChan, startedContainerChan, statsChan, errChan)
			drawContainerList(i)
			i++
			time.Sleep(time.Second / 2)
		default:
		}
	}
}
