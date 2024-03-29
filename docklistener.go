package main

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"

	. "github.com/byrnedo/dockdash/logger"
	goDocker "github.com/fsouza/go-dockerclient"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

type StatsResult struct {
	Container goDocker.Container
	Stats     goDocker.Stats
}

type StatsResultSlice []*StatsResult

func (p StatsResultSlice) Len() int {
	return len(p)
}

func (p StatsResultSlice) Less(i, j int) bool {
	return p[i].Container.State.StartedAt.After(p[j].Container.State.StartedAt)
}

func (p StatsResultSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

type ChartData struct {
	DataLabels []string
	Data       []float64
}

func (cd ChartData) Offset(offset int) ChartData {
	cd.Data = cd.Data[offset:]
	cd.DataLabels = cd.DataLabels[offset:]
	return cd
}

func (cd ChartData) UpdateBarChart(uiChart *widgets.BarChart) {

	uiChart.Data = cd.Data
	numBars := len(cd.Data)
	uiChart.BarColors = make([]ui.Color, numBars)
	uiChart.LabelStyles = make([]ui.Style, numBars)
	uiChart.NumStyles = make([]ui.Style, numBars)
	uiChart.Labels = make([]string, numBars)
	for i := 0; i < numBars; i++ {
		uiChart.BarColors[i] = ui.ColorWhite
		uiChart.LabelStyles[i] = ui.Style{Fg: ui.ColorWhite, Bg: ui.ColorClear}
		uiChart.NumStyles[i] = ui.Style{Fg: ui.ColorBlack}
		uiChart.Labels[i] = fmt.Sprintf("%3s", cd.DataLabels[i])
	}
}

type StatsMsg struct {
	CpuChart ChartData
	MemChart ChartData
}

type StatsListener struct {
	DockerClient         *goDocker.Client
	ctx                  context.Context
	cncl                 context.CancelFunc
	dockerEventChan      chan *goDocker.APIEvents
	statsResultsChan     chan StatsResult
	statsResultsDoneChan chan string
}

func (sl *StatsListener) Open(newContChan chan<- goDocker.Container, removeContChan chan<- string, drawStatsChan chan<- StatsMsg) {
	sl.ctx, sl.cncl = context.WithCancel(context.Background())

	sl.dockerEventChan = make(chan *goDocker.APIEvents, 10)
	sl.statsResultsChan = make(chan StatsResult)
	sl.statsResultsDoneChan = make(chan string)

	err := sl.DockerClient.AddEventListener(sl.dockerEventChan)
	if err != nil {
		panic("Failed to add event listener: " + err.Error())
	}

	go sl.statsRenderingRoutine(drawStatsChan)

	go sl.dockerEventRoutingRoutine(newContChan, removeContChan)

	containers, _ := sl.DockerClient.ListContainers(goDocker.ListContainersOptions{})
	Info.Println("Listing initial", len(containers), "containers as started")
	for _, cont := range containers {
		Info.Println("Marking", cont.ID, "as started")
		sl.dockerEventChan <- &goDocker.APIEvents{ID: cont.ID, Status: "start"}
	}

	Info.Println("stats listener open")
}

func (sl *StatsListener) Close() {
	if sl.cncl == nil {
		return
	}
	if err := sl.DockerClient.RemoveEventListener(sl.dockerEventChan); err != nil {
		panic(err)
	}
	sl.cncl()
	close(sl.dockerEventChan)
	close(sl.statsResultsChan)
	close(sl.statsResultsDoneChan)
}

func (sl *StatsListener) dockerEventRoutingRoutine(newContainerChan chan<- goDocker.Container, removeContainerChan chan<- string) {
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
		sl.statsResultsDoneChan <- id
	}

	for {
		select {
		case <-sl.ctx.Done():
			return
		case e := <-sl.dockerEventChan:
			if e == nil {
				continue
			}
			switch e.Status {
			case "start":
				Info.Println(e.ID, "started")
				cont, err := sl.DockerClient.InspectContainer(e.ID)
				if err != nil {
					Error.Println("Failed to inspect new container", e.ID, ":", err)
					continue
				}
				newContainerChan <- *cont
				sl.statsResultsChan <- StatsResult{*cont, goDocker.Stats{}}

				statsDoneChannels[cont.ID] = make(chan bool, 1)
				startsResultInterceptChannels[cont.ID] = make(chan *goDocker.Stats)
				startsResultInterceptDoneChannels[cont.ID] = make(chan bool)

				spinOffStatsInterceptor := func() {
					for {
						select {
						case <-sl.ctx.Done():
							return
						case stat := <-startsResultInterceptChannels[cont.ID]:
							if cont != nil {
								if stat == nil {
									stat = &goDocker.Stats{}
								}
								sl.statsResultsChan <- StatsResult{*cont, *stat}
							}
						case _ = <-startsResultInterceptDoneChannels[cont.ID]:
							return
						}
					}
				}
				go spinOffStatsInterceptor()

				Info.Println("Starting stats routine for", cont.ID)
				spinOffStatsListener := func() {
					if err := sl.DockerClient.Stats(goDocker.StatsOptions{ID: cont.ID, Stats: startsResultInterceptChannels[cont.ID], Stream: true, Done: statsDoneChannels[cont.ID]}); err != nil {
						Error.Println("Error starting statistics handler for id", cont.ID, ":", err.Error())
						startsResultInterceptDoneChannels[cont.ID] <- true
					}
					closeAndDeleteChannels(cont.ID)
				}
				go spinOffStatsListener()
			case "die":
				removeContainerChan <- e.ID
				sl.statsResultsDoneChan <- e.ID

				Info.Println("Stopping stats routine for", e.ID)
				statsDoneChannels[e.ID] <- true
				startsResultInterceptDoneChannels[e.ID] <- true
				closeAndDeleteChannels(e.ID)
			}
		}
	}
}

func (sl *StatsListener) statsRenderingRoutine(drawStatsChan chan<- StatsMsg) {

	var (
		statsList = make(map[string]*StatsResult)
	)

	for {
		select {
		case <-sl.ctx.Done():
			return
		case msg := <-sl.statsResultsChan:
			statsList[msg.Container.ID] = &msg
			statsCpuChart, statsMemChart := updateStatsBarCharts(statsList)
			drawStatsChan <- StatsMsg{*statsCpuChart, *statsMemChart}
		case id := <-sl.statsResultsDoneChan:
			delete(statsList, id)
			statsCpuChart, statsMemChart := updateStatsBarCharts(statsList)
			drawStatsChan <- StatsMsg{*statsCpuChart, *statsMemChart}
		}
	}
}

func updateStatsBarCharts(statsList map[string]*StatsResult) (statsCpuChart *ChartData, statsMemChart *ChartData) {
	statsCpuChart = &ChartData{}
	statsMemChart = &ChartData{}

	var (
		statsListLen = len(statsList)
		orderedList  = make(StatsResultSlice, statsListLen)
	)

	statsCpuChart.DataLabels = make([]string, statsListLen)
	statsCpuChart.Data = make([]float64, statsListLen)

	statsMemChart.DataLabels = make([]string, statsListLen)
	statsMemChart.Data = make([]float64, statsListLen)

	count := 0
	for _, nums := range statsList {
		orderedList[count] = nums
		count++
	}

	sort.Sort(orderedList)

	for count, stats := range orderedList {
		statsCpuChart.DataLabels[count] = strconv.Itoa(statsListLen - count)
		statsCpuChart.Data[count] = math.Round(calculateCPUPercent(&stats.Stats)*10) / 10

		statsMemChart.DataLabels[count] = strconv.Itoa(statsListLen - count)
		if stats.Stats.MemoryStats.Limit != 0 {
			statsMemChart.Data[count] = math.Round((float64(stats.Stats.MemoryStats.Usage)/float64(stats.Stats.MemoryStats.Limit)*100)*10) / 10
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
		cpuPercent = (cpuDelta / systemDelta) * float64(v.CPUStats.OnlineCPUs) * 100.0
	}
	return cpuPercent
}
