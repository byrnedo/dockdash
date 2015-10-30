package main

import (
	"fmt"
	"github.com/byrnedo/dockdash/docklistener"
	. "github.com/byrnedo/dockdash/logger"
	view "github.com/byrnedo/dockdash/view"
	goDocker "github.com/fsouza/go-dockerclient"
	ui "github.com/gizak/termui"
	flag "github.com/ogier/pflag"
	"io/ioutil"
	"os"
	//"time"
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
	drawChan            chan bool
	newContainerChan    chan goDocker.Container
	removeContainerChan chan string
	doneChan            chan bool
	uiEventChan         chan view.UIEvent
	drawStatsChan       chan docklistener.StatsMsg
)

var logFileFlag = flag.String("log-file", "", "Path to log file")
var dockerEndpoint = flag.String("docker-endpoint", "unix:/var/run/docker.sock", "Docker connection endpoint")
var helpFlag = flag.Bool("help", false, "help")

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

	docker, err := goDocker.NewClient(*dockerEndpoint)
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

	uiView.Align()

	newContainerChan = make(chan goDocker.Container)
	removeContainerChan = make(chan string)
	drawStatsChan = make(chan docklistener.StatsMsg)
	uiEventChan = make(chan view.UIEvent)
	drawChan = make(chan bool)

	// Statistics

	uiRoutine := func() {
		var (
			horizPosition int = 0
			offset        int = 0
			maxOffset     int = 0
			//lastStatsRender   time.Time = time.Time{}
			currentContainers = make(map[string]*goDocker.Container)
		)
		for {
			select {
			case e := <-uiEventChan:
				switch e {
				case view.Resize:
					uiView.ResetSize()
				case view.KeyQ, view.KeyCtrlC, view.KeyCtrlD:
					ui.StopLoop()
				case view.KeyArrowLeft:
					if horizPosition > 0 {
						horizPosition--
					}
					uiView.UpdateContainers(currentContainers, view.DockerInfoType(horizPosition), offset)
					ui.Render(ui.Body)
				case view.KeyArrowRight:
					if horizPosition < view.MaxHorizPosition {
						horizPosition++
					}
					uiView.UpdateContainers(currentContainers, view.DockerInfoType(horizPosition), offset)
					ui.Render(ui.Body)
				case view.KeyArrowDown:
					if offset < maxOffset && offset < view.MaxContainers {
						offset++
					}
					uiView.UpdateContainers(currentContainers, view.DockerInfoType(horizPosition), offset)
					ui.Render(ui.Body)
					//shift the list down
				case view.KeyArrowUp:
					if offset > 0 {
						offset--
					}
					uiView.UpdateContainers(currentContainers, view.DockerInfoType(horizPosition), offset)
					ui.Render(ui.Body)
					//shift the list up
				default:
					Info.Printf("Got unhandled key %+v\n", e)
				}
			case cont := <-newContainerChan:
				Info.Println("Got new containers event")
				Info.Printf("%d, %d, %d", offset, maxOffset, horizPosition)
				currentContainers[cont.ID] = &cont
				maxOffset = len(currentContainers) - 1
				uiView.UpdateContainers(currentContainers, view.DockerInfoType(horizPosition), offset)
				ui.Render(ui.Body)

			case removedContainerID := <-removeContainerChan:
				maxOffset = len(currentContainers) - 1
				if offset >= maxOffset {
					offset = maxOffset
				}
				Info.Printf("%d, %d, %d", offset, maxOffset, horizPosition)
				Info.Println("Got dead container event")
				delete(currentContainers, removedContainerID)

				uiView.UpdateContainers(currentContainers, view.DockerInfoType(horizPosition), offset)
				ui.Render(ui.Body)

			case newStatsCharts := <-drawStatsChan:
				//				if time.Now().Sub(lastStatsRender) > 500*time.Millisecond {
				uiView.UpdateStats(&newStatsCharts, offset)
				//					lastStatsRender = time.Now()
				//				}
			case <-drawChan:
				ui.Render(ui.Body)
			}
		}
	}

	//setup initial containers
	ui.Render(ui.Body)

	go uiRoutine()

	docklistener.Init(docker, newContainerChan, removeContainerChan, drawStatsChan)

	view.InitUIHandlers(uiEventChan)

	ui.Handle("/timer/1s", func(e ui.Event) {
		drawChan <- true
	})

	ui.Loop()

}
