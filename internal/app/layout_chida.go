package app

// Layout "chida": come vertical (L4), con CPU/GPU/DRAM/memoria come grafici storici,
// consumi a sinistra e ventole/temperature sotto i processi.
// Vive in questo file e si registra da solo, così i fix di upstream si prendono
// senza conflitti.

import (
	"fmt"
	"strings"

	ui "github.com/metaspartan/gotui/v5"
	w "github.com/metaspartan/gotui/v5/widgets"
)

const LayoutChida = "chida"

func init() {
	layoutOrder = append([]string{LayoutChida}, layoutOrder...) // primo con "l"
	namedLayoutSetters[LayoutChida] = setChidaLayoutGrid
}

var chidaCPU, chidaGPU, chidaBW, chidaMem *tailChart
var chidaFans, chidaTemps *textPanel

func setChidaLayoutGrid() {
	if chidaCPU == nil {
		chidaCPU = &tailChart{cpuHistoryChart, func() string { return cpuGauge.Title }, nil}
		chidaGPU = &tailChart{gpuHistoryChart, func() string { return gpuGauge.Title }, nil}
		chidaBW = &tailChart{memBWHistoryChart, func() string {
			return fmt.Sprintf("DRAM  R %.1f GB/s · W %.1f GB/s", lastCPUMetrics.DRAMReadBW, lastCPUMetrics.DRAMWriteBW)
		}, []string{"R", "W"}}
		chidaMem = &tailChart{memoryHistoryChart, nil, []string{"Usata", "Swap"}}
		chidaFans = newTextPanel(chidaFanText)
		chidaTemps = newTextPanel(chidaTempText)
	}
	grid.Set(
		ui.NewRow(1.0,
			ui.NewCol(0.4,
				ui.NewRow(1.25/8, chidaCPU),
				ui.NewRow(1.25/8, chidaGPU),
				ui.NewRow(1.0/8, chidaBW),
				ui.NewRow(1.5/8, chidaMem),
				ui.NewRow(1.0/8, NetworkInfo),
				ui.NewRow(2.0/8,
					ui.NewCol(1.0/2, PowerChart),
					ui.NewCol(1.0/2, sparklineGroup),
				),
			),
			ui.NewCol(0.6,
				ui.NewRow(0.68, processList),
				ui.NewRow(0.32,
					ui.NewCol(1.0/2, chidaFans),
					ui.NewCol(1.0/2, chidaTemps),
				),
			),
		),
	)
}

// tailChart disegna solo gli ultimi punti che entrano nel riquadro: gli update di
// upstream tagliano la storia sulla larghezza dei loro layout e StepChart disegna
// da sinistra, quindi senza taglio si vedrebbe la parte vecchia.
type tailChart struct {
	*w.StepChart
	title  func() string
	series []string // nomi delle linee, davanti alle etichette
}

func (t *tailChart) Draw(buf *ui.Buffer) {
	if n := t.Inner.Dx(); n > 0 {
		for i, d := range t.Data {
			if len(d) > n {
				t.Data[i] = d[len(d)-n:]
			}
		}
	}
	if t.title != nil {
		t.Title = t.title()
	}
	for i, name := range t.series {
		if i < len(t.DataLabels) && !strings.HasPrefix(t.DataLabels[i], name) {
			t.DataLabels[i] = name + " " + t.DataLabels[i]
		}
	}
	// I temi standard forzano un solo colore per tutte le linee: la seconda va distinta.
	if len(t.series) > 1 {
		second := ui.ColorMagenta
		if t.BorderStyle.Fg == second {
			second = ui.ColorYellow
		}
		t.LineColors = []ui.Color{t.BorderStyle.Fg, second}
	}
	t.StepChart.Draw(buf)
}

// textPanel rigenera titolo e testo a ogni disegno, con lo stile del pannello rete (tema).
type textPanel struct {
	*w.Paragraph
	text func(width int) (title, text string)
}

func newTextPanel(text func(int) (string, string)) *textPanel {
	return &textPanel{w.NewParagraph(), text}
}

func (p *textPanel) Draw(buf *ui.Buffer) {
	p.BorderStyle, p.TitleStyle, p.TextStyle = NetworkInfo.BorderStyle, NetworkInfo.TitleStyle, NetworkInfo.TextStyle
	p.Title, p.Text = p.text(p.Inner.Dx())
	p.Paragraph.Draw(buf)
}

func chidaThemeColor() string {
	c := currentConfig.Theme
	if c == "" {
		c = "green"
	}
	if IsLightMode && c == "white" {
		c = "black"
	}
	return c
}

// Stesso contenuto del pannello ventole di L21, con lo stato di fanboost nel titolo.
func chidaFanText(int) (string, string) {
	title := "Ventole · AUTO (curva Apple)"
	for _, f := range lastCPUMetrics.Fans {
		if f.Mode == 1 {
			title = "Ventole · BOOST (manuale)"
		}
	}
	return title, buildFanStatusText(chidaThemeColor())
}

// Riga con i massimi che usa fanboost (SSD da NVMe SMART), poi i gruppi di L21.
func chidaTempText(int) (string, string) {
	var cpu, gpu, mem, ssd, bat float64
	for _, s := range lastCPUMetrics.TempSensors {
		k, v := s.Key, s.Value
		if v <= 0 || v >= 150 {
			continue
		}
		switch {
		case strings.HasPrefix(k, "Tp"), strings.HasPrefix(k, "Te"):
			cpu = max(cpu, v)
		case strings.HasPrefix(k, "Tg"):
			gpu = max(gpu, v)
		case strings.HasPrefix(k, "Tm"):
			mem = max(mem, v)
		case strings.HasPrefix(k, "Nv"):
			ssd = max(ssd, v)
		case k == "TB0T", k == "TB1T", k == "TB2T":
			bat = max(bat, v)
		}
	}
	t := func(v float64) string {
		if v == 0 {
			return "n/d"
		}
		return formatTemp(v)
	}
	tc := chidaThemeColor()
	lines := []string{fmt.Sprintf("[fanboost max](fg:%s,mod:bold)  CPU %s  GPU %s  MEM %s  SSD %s  BAT %s", tc, t(cpu), t(gpu), t(mem), t(ssd), t(bat)), ""}
	lines = append(lines, buildGroupedTempLines(lastCPUMetrics.TempSensors, tc)...)
	return "Temperature", strings.Join(lines, "\n")
}
