package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/byrnedo/dockdash/docklistener"
	. "github.com/byrnedo/dockdash/logger"
	view "github.com/byrnedo/dockdash/view"
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
	uiEventChan         chan view.UIEvent
	drawStatsChan       chan docklistener.StatsMsg
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

	var uiView = view.NewView()

	uiView.SetLayout()

	newContainerChan = make(chan goDocker.Container)
	removeContainerChan = make(chan string)
	drawStatsChan = make(chan docklistener.StatsMsg)
	uiEventChan = make(chan view.UIEvent)

	// Statistics

	uiRoutine := func() {

		drawTicker := time.NewTicker(1 * time.Second)
		defer drawTicker.Stop()

		var (
			inspectMode   bool = false
			horizPosition int  = 0
			offset        int  = 0
			maxOffset     int  = 0
			currentStats  *docklistener.StatsMsg
			//lastStatsRender   time.Time = time.Time{}
			currentContainers = make(map[string]*goDocker.Container)
			ticker            = time.NewTicker(1 * time.Second)
		)
		for {
			select {
			case e := <-uiEventChan:
				switch e {
				case view.Resize:
					uiView.ResetSize()
				case view.KeyQ, view.KeyCtrlC, view.KeyCtrlD:
					ui.Close()
					os.Exit(0)
				case view.KeyArrowLeft:
					if horizPosition > 0 {
						horizPosition--
					}
					uiView.RenderContainers(currentContainers, view.DockerInfoType(horizPosition), offset, inspectMode)
				case view.KeyArrowRight:
					if horizPosition < view.MaxHorizPosition {
						horizPosition++
					}
					uiView.RenderContainers(currentContainers, view.DockerInfoType(horizPosition), offset, inspectMode)
				case view.KeyArrowDown:
					if offset < maxOffset && offset < view.MaxContainers {
						offset++
					}
					uiView.RenderContainers(currentContainers, view.DockerInfoType(horizPosition), offset, inspectMode)
					//shift the list down
				case view.KeyArrowUp:
					if offset > 0 {
						offset--
					}
					uiView.RenderContainers(currentContainers, view.DockerInfoType(horizPosition), offset, inspectMode)
					//shift the list up
				case view.KeyI:
					inspectMode = !inspectMode
					uiView.RenderContainers(currentContainers, view.DockerInfoType(horizPosition), offset, inspectMode)
				default:
					Info.Printf("Got unhandled key %+v\n", e)
				}
			case cont := <-newContainerChan:
				Info.Println("Got new containers event")
				Info.Printf("%d, %d, %d", offset, maxOffset, horizPosition)
				currentContainers[cont.ID] = &cont
				maxOffset = len(currentContainers) - 1
				uiView.RenderContainers(currentContainers, view.DockerInfoType(horizPosition), offset, inspectMode)

			case removedContainerID := <-removeContainerChan:
				maxOffset = len(currentContainers) - 1
				if offset >= maxOffset {
					offset = maxOffset
				}
				Info.Printf("%d, %d, %d", offset, maxOffset, horizPosition)
				Info.Println("Got dead container event")
				delete(currentContainers, removedContainerID)

				uiView.RenderContainers(currentContainers, view.DockerInfoType(horizPosition), offset, inspectMode)

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

	//setup initial containers
	uiView.Render()

	go uiRoutine()

	docklistener.Init(docker, newContainerChan, removeContainerChan, drawStatsChan)

	uiEvents := ui.PollEvents()
	for {
		select {
		case e := <-uiEvents:
			Info.Printf("%s - %s\n", e.ID, e.Type)
			switch e.ID {
			case "q":
				uiEventChan <- view.KeyQ
			case "<C-c>":
				uiEventChan <- view.KeyCtrlC
			case "<C-d>":
				uiEventChan <- view.KeyCtrlD
			case "<Left>":
				uiEventChan <- view.KeyArrowLeft
			case "<Right>":
				uiEventChan <- view.KeyArrowRight
			case "<Down>":
				uiEventChan <- view.KeyArrowDown
			case "<Up>":
				uiEventChan <- view.KeyArrowUp
			case "<Resize>":
				uiEventChan <- view.Resize
			case "i":
				uiEventChan <- view.KeyI
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
