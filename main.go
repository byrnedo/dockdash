package main

import (
	"github.com/byrnedo/dockdash/dockerClient"
	goDocker "github.com/fsouza/go-dockerclient"
	ui "github.com/gizak/termui"
	"strings"
	"time"
)

func createExitBar() (p *ui.Par) {
	p = ui.NewPar(":PRESS q TO QUIT DEMO")
	p.Height = 3
	p.TextFgColor = ui.ColorWhite
	p.Border.Label = "Text Box"
	p.Border.FgColor = ui.ColorCyan
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

func getNamesAndImagesOfRunning(cl *dockerClient.DockerClient) ([]string, []string) {
	containers, _ := cl.ListContainers(goDocker.ListContainersOptions{})
	names := make([]string, len(containers))
	images := make([]string, len(containers))
	for i, cont := range containers {
		names[i] = strings.Join(cont.Names, "")
		images[i] = cont.Image
	}
	return names, images
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

	p := createExitBar()

	containerListOfNames := createContainerList()
	containerListOfImages := createContainerList()

	lc := createDockerLineChart()

	lc.Data = make([]float64, 0, 0)

	ui.Body.AddRows(
		ui.NewRow(
			ui.NewCol(6, 0, p),
		),
		ui.NewRow(
			ui.NewCol(3, 0, containerListOfNames),
			ui.NewCol(3, 0, containerListOfImages),
		),
	)

	// calculate layout
	ui.Body.Align()

	draw := func(t int) {

		names, images := getNamesAndImagesOfRunning(docker)
		containerListOfNames.Items = names
		containerListOfImages.Items = images
		containerListOfNames.Height = len(names) + 1
		containerListOfImages.Height = len(images) + 1

		lc.Data = append(lc.Data, float64(len(names)))
		ui.Render(p, containerListOfNames, containerListOfImages, lc)
	}

	evt := ui.EventCh()

	i := 0
	for {
		select {
		case e := <-evt:
			if e.Type == ui.EventKey && e.Ch == 'q' {
				return
			}
			if e.Type == ui.EventResize {
				ui.Body.Width = ui.TermWidth()
				ui.Body.Align()
				go func() { evt <- ui.Event{} }()
			}
		default:
			draw(i)
			i++
			time.Sleep(time.Second / 2)
		}
	}
}
