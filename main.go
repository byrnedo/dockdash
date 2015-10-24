package main

import (
	"./docklistener"
	. "./logger"
	goDocker "github.com/fsouza/go-dockerclient"
	ui "github.com/gizak/termui"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

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
	newContainerChan     = make(chan *goDocker.Container)
	removeContainerChan  = make(chan string)
	listStartOffsetChan  = make(chan int)
	horizPositionChan    = make(chan int)
	maxOffsetChan        = make(chan int)
	deadContainerChan    = make(chan string)
	startedContainerChan = make(chan string)
	doneChan             = make(chan bool)
	uiEventChan          = ui.EventCh()
	drawContainersChan   = make(chan *ContainersMsg)
	drawStatsChan        = make(chan *docklistener.StatsMsg)

	startGatheringStatisticsChan = make(chan *goDocker.Container)
	stopGatheringStatisticsChan  = make(chan string)
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

		names[index-offset] = "(" + strconv.Itoa(index+1) + ") " + cont.ID[:12] + " " + strings.TrimLeft(cont.Name, "/")
		switch infoType {
		case ImageInfo:
			info[index-offset] = cont.Config.Image
		case PortInfo:
			info[index-offset] = createPortsString(cont.NetworkSettings.Ports)
		case BindInfo:
			info[index-offset] = strings.TrimRight(strings.Join(cont.HostConfig.Binds, ","), ",")
		case CommandInfo:
			info[index-offset] = cont.Path + " " + strings.Join(cont.Args, " ")
		case EnvInfo:
			info[index-offset] = strings.TrimRight(strings.Join(cont.Config.Env, ","), ",")
		case EntrypointInfo:
			info[index-offset] = strings.Join(cont.Config.Entrypoint, " ")
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

func main() {
	var logPath = "/tmp/dockdash.log"
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic("Failed to open log file " + logPath + ":" + err.Error())
	}

	InitLog(ioutil.Discard, file, file, file)

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

	docklistener.Init(docker, newContainerChan, removeContainerChan, drawStatsChan)

	// Statistics

	uiRoutine := func() {
		var (
			horizPosition     int       = 0
			offset            int       = 0
			maxOffset         int       = 0
			lastStatsRender   time.Time = time.Time{}
			currentContainers           = make(map[string]*goDocker.Container)
		)
		renderContainers := func(containers map[string]*goDocker.Container, infoType DockerInfoType, listOffset int) {
			names, info := getNameAndInfoOfContainers(containers, listOffset, infoType)
			var height = len(names) + 2
			uiView.NameList.Height = height
			uiView.NameList.Items = names
			uiView.InfoList.Height = height
			uiView.InfoList.Items = info
			uiView.InfoList.Border.Label = InfoHeaders[infoType]
			uiView.Render()
		}
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
						case ui.KeyArrowRight:
							if horizPosition < MaxHorizPosition {
								horizPosition++
							}
						case ui.KeyArrowDown:
							if offset < maxOffset && offset < MaxContainers {
								offset++
							}
							//shift the list down
						case ui.KeyArrowUp:
							if offset > 0 {
								offset--
							}
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
				currentContainers[cont.ID] = cont
				maxOffset = len(currentContainers) - 1

				renderContainers(currentContainers, DockerInfoType(horizPosition), offset)

			case removedContainerID := <-removeContainerChan:
				Info.Println("Got dead container event")
				delete(currentContainers, removedContainerID)
				maxOffset = len(currentContainers) - 1

				renderContainers(currentContainers, DockerInfoType(horizPosition), offset)

			case newStatsCharts := <-drawStatsChan:

				uiView.CpuChart.Data = newStatsCharts.CpuChart.Data[offset:]
				uiView.CpuChart.DataLabels = newStatsCharts.CpuChart.DataLabels[offset:]
				uiView.MemChart.Data = newStatsCharts.MemChart.Data[offset:]
				uiView.MemChart.DataLabels = newStatsCharts.MemChart.DataLabels[offset:]
				if time.Now().Sub(lastStatsRender) > 500*time.Millisecond {
					Info.Println("Got draw stats event")
					uiView.Render()
					lastStatsRender = time.Now()
				}
			}
		}
	}
	go uiRoutine()

	//setup initial containers
	uiView.Render()

	<-doneChan

}
