package main

import (
	"flag"
	"github.com/byrnedo/dockdash/docklistener"
	. "github.com/byrnedo/dockdash/logger"
	goDocker "github.com/fsouza/go-dockerclient"
	ui "github.com/gizak/termui"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"
)

var logFile = flag.String("log-file", "", "Path to log file")

type ListData struct {
	Label string
	Items []string
}

type ContainersMsg struct {
	Left  *ListData
	Right *ListData
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

var (
	newContainerChan    chan *goDocker.Container
	removeContainerChan chan string
	doneChan            chan bool
	uiEventChan         <-chan ui.Event
	drawStatsChan       chan *docklistener.StatsMsg
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

func main() {
	if len(*logFile) > 0 {
		file, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			panic("Failed to open log file " + *logFile + ":" + err.Error())
		}
		InitLog(ioutil.Discard, file, file, file)
	} else {
		InitLog(ioutil.Discard, ioutil.Discard, ioutil.Discard, ioutil.Discard)
	}

	docker, err := goDocker.NewClientFromEnv()
	if err != nil {
		panic(err)
	}

	err = ui.Init()
	if err != nil {
		panic(err)
	}

	defer ui.Close()

	var uiView = NewView()

	uiView.SetLayout()

	uiView.Align()

	newContainerChan = make(chan *goDocker.Container)
	removeContainerChan = make(chan string)
	doneChan = make(chan bool)
	uiEventChan = ui.EventCh()
	drawStatsChan = make(chan *docklistener.StatsMsg)

	// Statistics

	uiRoutine := func() {
		var (
			horizPosition     int       = 0
			offset            int       = 0
			maxOffset         int       = 0
			lastStatsRender   time.Time = time.Time{}
			currentContainers           = make(map[string]*goDocker.Container)
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
						case ui.KeyCtrlC, ui.KeyCtrlD:
							doneChan <- true
						case ui.KeyArrowLeft:
							if horizPosition > 0 {
								horizPosition--
							}
							uiView.RenderContainers(currentContainers, DockerInfoType(horizPosition), offset)
						case ui.KeyArrowRight:
							if horizPosition < MaxHorizPosition {
								horizPosition++
							}
							uiView.RenderContainers(currentContainers, DockerInfoType(horizPosition), offset)
						case ui.KeyArrowDown:
							if offset < maxOffset && offset < MaxContainers {
								offset++
							}
							uiView.RenderContainers(currentContainers, DockerInfoType(horizPosition), offset)
							//shift the list down
						case ui.KeyArrowUp:
							if offset > 0 {
								offset--
							}
							uiView.RenderContainers(currentContainers, DockerInfoType(horizPosition), offset)
							//shift the list up
						default:
							Info.Printf("Got unhandled key %d\n", e.Key)
						}
					}
				}
				if e.Type == ui.EventResize {
					uiView.ResetSize()
				}
				uiView.Render()
			case cont := <-newContainerChan:
				Info.Println("Got new containers event")
				Info.Printf("%d, %d, %d", offset, maxOffset, horizPosition)
				currentContainers[cont.ID] = cont
				maxOffset = len(currentContainers) - 1
				uiView.RenderContainers(currentContainers, DockerInfoType(horizPosition), offset)

			case removedContainerID := <-removeContainerChan:
				maxOffset = len(currentContainers) - 1
				if offset >= maxOffset {
					offset = maxOffset
				}
				Info.Printf("%d, %d, %d", offset, maxOffset, horizPosition)
				Info.Println("Got dead container event")
				delete(currentContainers, removedContainerID)

				uiView.RenderContainers(currentContainers, DockerInfoType(horizPosition), offset)

			case newStatsCharts := <-drawStatsChan:

				uiView.CpuChart.Data = newStatsCharts.CpuChart.Data[offset:]
				uiView.CpuChart.DataLabels = newStatsCharts.CpuChart.DataLabels[offset:]
				uiView.MemChart.Data = newStatsCharts.MemChart.Data[offset:]
				uiView.MemChart.DataLabels = newStatsCharts.MemChart.DataLabels[offset:]
			default:
				if time.Now().Sub(lastStatsRender) > 500*time.Millisecond {
					uiView.Render()
					lastStatsRender = time.Now()
				}
			}
		}
	}
	go uiRoutine()

	docklistener.Init(docker, newContainerChan, removeContainerChan, drawStatsChan)

	//setup initial containers
	uiView.Render()

	<-doneChan

}
