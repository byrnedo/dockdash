package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	. "github.com/byrnedo/dockdash/logger"
	goDocker "github.com/fsouza/go-dockerclient"
	ui "github.com/gizak/termui/v3"
	flag "github.com/ogier/pflag"
)

type ListData struct {
	Label string
	Items []string
}

type ContainersMsg struct {
	Left  *ListData
	Right *ListData
}

var (
	newContainerChan    chan goDocker.Container
	removeContainerChan chan string
	doneChan            chan bool
	uiEventChan         chan UIEvent
	drawStatsChan       chan StatsMsg
)

var logFileFlag = flag.String("log-file", "", "Path to log file")
var dockerEndpoint = flag.String("docker-endpoint", "", "Docker connection endpoint")
var helpFlag = flag.Bool("help", false, "help")
var versionFlag = flag.Bool("version", false, "print version")

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dockdash [options]\n\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *helpFlag {
		flag.Usage()
		os.Exit(1)
	}
	if *versionFlag {
		fmt.Println(VERSION)
		os.Exit(0)
	}
}

func main() {

	if len(*logFileFlag) > 0 {
		file, err := os.OpenFile(*logFileFlag, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			panic("Failed to open log file " + *logFileFlag + ":" + err.Error())
		}
		InitLog(ioutil.Discard, file, file, file)
	} else {
		InitLog(ioutil.Discard, ioutil.Discard, ioutil.Discard, ioutil.Discard)
	}

	var (
		docker *goDocker.Client
		err    error
	)

	if len(*dockerEndpoint) > 0 {
		docker, err = goDocker.NewClient(*dockerEndpoint)
	} else {
		docker, err = goDocker.NewClientFromEnv()
	}

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

	newContainerChan = make(chan goDocker.Container)
	removeContainerChan = make(chan string)
	drawStatsChan = make(chan StatsMsg)
	uiEventChan = make(chan UIEvent)

	// Statistics

	sl := &StatsListener{DockerClient: docker}

	//setup initial containers
	uiView.Render()

	go handleUiEvents()
	Info.Println("ui event loop running")

	Info.Println("starting render routine")

	drawTicker := time.NewTicker(1 * time.Second)
	defer drawTicker.Stop()

	go func() {
		time.Sleep(500 * time.Millisecond)
		Info.Println("opening stats listener")
		sl.Open(newContainerChan, removeContainerChan, drawStatsChan)
		Info.Println("stats listener open")
	}()

	mainLoop(uiView, sl)

}

func mainLoop(uiView *View, sl *StatsListener) {

	var (
		inspectMode       bool = false
		horizPosition     int  = 0
		offset            int  = 0
		maxOffset         int  = 0
		currentStats      *StatsMsg
		currentContainers = make(map[string]*goDocker.Container)
		ticker            = time.NewTicker(1 * time.Second)
	)
	for {
		select {
		case e := <-uiEventChan:
			switch e {
			case Resize:
				uiView.ResetSize()
			case KeyQ, KeyCtrlC, KeyCtrlD:
				sl.Close()
				ui.Close()
				os.Exit(0)
			case KeyArrowLeft:
				if horizPosition > 0 {
					horizPosition--
				}
				uiView.RenderContainers(currentContainers, DockerInfoType(horizPosition), offset, inspectMode)
			case KeyArrowRight:
				if horizPosition < MaxHorizPosition {
					horizPosition++
				}
				uiView.RenderContainers(currentContainers, DockerInfoType(horizPosition), offset, inspectMode)
			case KeyArrowDown:
				if offset < maxOffset && offset < MaxContainers {
					offset++
				}
				uiView.RenderContainers(currentContainers, DockerInfoType(horizPosition), offset, inspectMode)
				//shift the list down
			case KeyArrowUp:
				if offset > 0 {
					offset--
				}
				uiView.RenderContainers(currentContainers, DockerInfoType(horizPosition), offset, inspectMode)
				//shift the list up
			case KeyI:
				inspectMode = !inspectMode
				uiView.RenderContainers(currentContainers, DockerInfoType(horizPosition), offset, inspectMode)
			default:
				Info.Printf("Got unhandled key %+v\n", e)
			}
		case cont := <-newContainerChan:
			Info.Println("Got new containers event")
			Info.Printf("%d, %d, %d", offset, maxOffset, horizPosition)
			currentContainers[cont.ID] = &cont
			maxOffset = len(currentContainers) - 1
			uiView.RenderContainers(currentContainers, DockerInfoType(horizPosition), offset, inspectMode)

		case removedContainerID := <-removeContainerChan:
			maxOffset = len(currentContainers) - 1
			if offset >= maxOffset {
				offset = maxOffset
			}
			Info.Printf("%d, %d, %d", offset, maxOffset, horizPosition)
			Info.Println("Got dead container event")
			delete(currentContainers, removedContainerID)

			uiView.RenderContainers(currentContainers, DockerInfoType(horizPosition), offset, inspectMode)

		case newStatsCharts := <-drawStatsChan:
			//				if time.Now().Sub(lastStatsRender) > 500*time.Millisecond {
			currentStats = &newStatsCharts
			uiView.UpdateStats(&newStatsCharts, offset)
			//					lastStatsRender = time.Now()
			//				}
		case <-ticker.C:
			var (
				numCons  = len(currentContainers)
				totalCpu = 0.0
				totalMem = 0.0
			)
			if currentStats != nil {
				totalCpu = sum(currentStats.CpuChart.Data...)
				totalMem = sum(currentStats.MemChart.Data...)
			}

			uiView.InfoBar.Text = fmt.Sprintf(" Cons:%d  Total CPU:%d%%  Total Mem:%d%%", numCons, int(totalCpu), int(totalMem))
			uiView.Render()
		}
	}
}

func handleUiEvents() {
	uiEvents := ui.PollEvents()
	for {
		select {
		case e := <-uiEvents:
			Info.Printf("%s - %s\n", e.ID, e.Type)
			switch e.ID {
			case "q":
				uiEventChan <- KeyQ
			case "<C-c>":
				uiEventChan <- KeyCtrlC
			case "<C-d>":
				uiEventChan <- KeyCtrlD
			case "<Left>":
				uiEventChan <- KeyArrowLeft
			case "<Right>":
				uiEventChan <- KeyArrowRight
			case "<Down>":
				uiEventChan <- KeyArrowDown
			case "<Up>":
				uiEventChan <- KeyArrowUp
			case "<Resize>":
				uiEventChan <- Resize
			case "i":
				uiEventChan <- KeyI
			}
		}
	}
}

func sum(nums ...float64) float64 {
	total := 0.0
	for _, num := range nums {
		total += num
	}
	return total
}
