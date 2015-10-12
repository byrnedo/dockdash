package main

import (
	ui "github.com/gizak/termui"
	"os/exec"
	"strings"
	"time"
)

const (
	LEFT_COL_WIDTH = 75

	CPU_GAUGE_START_X = 0
	CPU_GAUGE_START_Y = 4
	CPU_GAUGE_HEIGHT  = 3

	MEM_GAUGE_START_X = 0
	MEM_GAUGE_START_Y = CPU_GAUGE_START_Y + CPU_GAUGE_HEIGHT + 1
	MEM_GAUGE_HEIGHT  = 3

	LC_START_X = 0
	LC_START_Y = MEM_GAUGE_START_Y + MEM_GAUGE_HEIGHT + 1
	LC_HEIGHT  = 11

	LIST_START_X = 0
	LIST_START_Y = LC_START_Y + LC_HEIGHT + 1
	LIST_HEIGHT  = 14
)

func getDockerPs() []string {
	listBytes, err := exec.Command("docker", "ps", "--format", "'{{.ID}}\t{{.Image}}'").Output()
	if err != nil {
		if len(listBytes) != 0 {
			return strings.Split(string(listBytes), "\n")
		}
		return []string{err.Error()}
	} else {
		strippedStr := strings.Replace(string(listBytes), "dockerregistry.pagero.local", "d.p.l", -1)
		return strings.Split(strippedStr, "\n")
	}
}

func createExitBar() (p *ui.Par) {
	p = ui.NewPar(":PRESS q TO QUIT DEMO")
	p.Height = 3
	p.Width = LEFT_COL_WIDTH
	p.TextFgColor = ui.ColorWhite
	p.Border.Label = "Text Box"
	p.Border.FgColor = ui.ColorCyan
	return
}

func createCPUGauge() *ui.Gauge {
	cpuGauge := ui.NewGauge()
	cpuGauge.Percent = 50
	cpuGauge.Width = LEFT_COL_WIDTH
	cpuGauge.Height = CPU_GAUGE_HEIGHT
	cpuGauge.Y = CPU_GAUGE_START_Y
	cpuGauge.X = CPU_GAUGE_START_X
	cpuGauge.Border.Label = "CPU Usage"
	cpuGauge.BarColor = ui.ColorRed
	cpuGauge.Border.FgColor = ui.ColorWhite
	cpuGauge.Border.LabelFgColor = ui.ColorCyan
	return cpuGauge
}

func createMemGauge() *ui.Gauge {
	memGauge := ui.NewGauge()
	memGauge.Percent = 50
	memGauge.Width = LEFT_COL_WIDTH
	memGauge.Height = MEM_GAUGE_HEIGHT
	memGauge.Y = MEM_GAUGE_START_Y
	memGauge.X = MEM_GAUGE_START_X
	memGauge.Border.Label = "Mem. Usage"
	memGauge.BarColor = ui.ColorRed
	memGauge.Border.FgColor = ui.ColorWhite
	memGauge.Border.LabelFgColor = ui.ColorCyan
	return memGauge
}

func createContainerList() *ui.List {
	list := ui.NewList()
	list.ItemFgColor = ui.ColorYellow
	list.Border.Label = "Containers"
	list.Height = LIST_HEIGHT
	list.Width = LEFT_COL_WIDTH
	list.Y = LIST_START_Y
	list.X = LIST_START_X
	return list
}

func createDockerLineChart() *ui.LineChart {
	lc := ui.NewLineChart()
	lc.Border.Label = "Container Numbers"
	//lc.Data = sinps
	lc.Width = LEFT_COL_WIDTH
	lc.Height = LC_HEIGHT
	lc.X = 0
	lc.Y = LC_START_Y
	lc.AxesColor = ui.ColorWhite
	lc.LineColor = ui.ColorRed | ui.AttrBold
	lc.Mode = "line"
	return lc
}

func main() {
	err := ui.Init()
	if err != nil {
		panic(err)
	}
	defer ui.Close()

	p := createExitBar()

	cpuGauge := createCPUGauge()

	memGauge := createMemGauge()

	containerList := createContainerList()

	/*
	 *spark := ui.Sparkline{}
	 *spark.Height = 1
	 *spark.Title = "srv 0:"
	 *spdata := []int{4, 2, 1, 6, 3, 9, 1, 4, 2, 15, 14, 9, 8, 6, 10, 13, 15, 12, 10, 5, 3, 6, 1, 7, 10, 10, 14, 13, 6, 4, 2, 1, 6, 3, 9, 1, 4, 2, 15, 14, 9, 8, 6, 10, 13, 15, 12, 10, 5, 3, 6, 1, 7, 10, 10, 14, 13, 6, 4, 2, 1, 6, 3, 9, 1, 4, 2, 15, 14, 9, 8, 6, 10, 13, 15, 12, 10, 5, 3, 6, 1, 7, 10, 10, 14, 13, 6, 4, 2, 1, 6, 3, 9, 1, 4, 2, 15, 14, 9, 8, 6, 10, 13, 15, 12, 10, 5, 3, 6, 1, 7, 10, 10, 14, 13, 6}
	 *spark.Data = spdata
	 *spark.LineColor = ui.ColorCyan
	 *spark.TitleColor = ui.ColorWhite
	 */

	/*
	 *spark1 := ui.Sparkline{}
	 *spark1.Height = 1
	 *spark1.Title = "srv 1:"
	 *spark1.Data = spdata
	 *spark1.TitleColor = ui.ColorWhite
	 *spark1.LineColor = ui.ColorRed
	 */

	/*
	 *sp := ui.NewSparklines( [>spark,<] spark1)
	 *sp.Width = 25
	 *sp.Height = 7
	 *sp.Border.Label = "Sparkline"
	 *sp.Y = 4
	 *sp.X = 25
	 */

	/*
	 *sinps := (func() []float64 {
	 *    n := 220
	 *    ps := make([]float64, n)
	 *    for i := range ps {
	 *        ps[i] = 1 + math.Sin(float64(i)/5)
	 *    }
	 *    return ps
	 *})()
	 */

	lc := createDockerLineChart()

	/*
	 *    bc := ui.NewBarChart()
	 *    bcdata := []int{3, 2, 5, 3, 9, 5, 3, 2, 5, 8, 3, 2, 4, 5, 3, 2, 5, 7, 5, 3, 2, 6, 7, 4, 6, 3, 6, 7, 8, 3, 6, 4, 5, 3, 2, 4, 6, 4, 8, 5, 9, 4, 3, 6, 5, 3, 6}
	 *    bclabels := []string{"S0", "S1", "S2", "S3", "S4", "S5"}
	 *    bc.Border.Label = "Bar Chart"
	 *    bc.Width = 26
	 *    bc.Height = 10
	 *    bc.X = 76
	 *    bc.Y = 0
	 *    bc.DataLabels = bclabels
	 *    bc.BarColor = ui.ColorGreen
	 *    bc.NumColor = ui.ColorBlack
	 *
	 *    lc1 := ui.NewLineChart()
	 *    lc1.Border.Label = "braille-mode Line Chart"
	 *    lc1.Data = sinps
	 *    lc1.Width = 26
	 *    lc1.Height = 11
	 *    lc1.X = 76
	 *    lc1.Y = 14
	 *    lc1.AxesColor = ui.ColorWhite
	 *    lc1.LineColor = ui.ColorYellow | ui.AttrBold
	 *
	 *    p1 := ui.NewPar("Hey!\nI am a borderless block!")
	 *    p1.HasBorder = false
	 *    p1.Width = 26
	 *    p1.Height = 2
	 *    p1.TextFgColor = ui.ColorMagenta
	 *    p1.X = 77
	 *    p1.Y = 11
	 */

	lc.Data = make([]float64, 0, 0)

	draw := func(t int) {
		cpuGauge.Percent = t % 101
		containerList.Items = getDockerPs()
		containerList.Height = len(containerList.Items) + 1
		/*
		 *sp.Lines[0].Data = spdata[:30+t%50]
		 *sp.Lines[1].Data = spdata[:35+t%50]
		 */

		lc.Data = append(lc.Data, float64(len(containerList.Items)))
		/*
		 *lc1.Data = sinps[2*t:]
		 *bc.Data = bcdata[t/2%10:]
		 */
		ui.Render(p, containerList, memGauge, cpuGauge, lc)
	}

	evt := ui.EventCh()

	i := 0
	for {
		select {
		case e := <-evt:
			if e.Type == ui.EventKey && e.Ch == 'q' {
				return
			}
		default:
			draw(i)
			i++
			time.Sleep(time.Second / 2)
		}
	}
}
