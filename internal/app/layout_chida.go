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
	layoutOrder = append(layoutOrder, LayoutChida)
	namedLayoutSetters[LayoutChida] = setChidaLayoutGrid
}

var chidaCPU, chidaGPU, chidaBW, chidaMem *tailChart
var chidaFans, chidaTemps *textPanel

func setChidaLayoutGrid() {
	if chidaCPU == nil {
		chidaCPU = &tailChart{cpuHistoryChart, func() string { return cpuGauge.Title }}
		chidaGPU = &tailChart{gpuHistoryChart, func() string { return gpuGauge.Title }}
		chidaBW = &tailChart{memBWHistoryChart, nil}
		chidaMem = &tailChart{memoryHistoryChart, nil}
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
				ui.NewRow(0.9, processList),
				ui.NewRow(0.1,
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
	title func() string
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

func chidaFanText(width int) (string, string) {
	fans := lastCPUMetrics.Fans
	if len(fans) == 0 {
		return "Ventole", "nessuna ventola"
	}
	var b strings.Builder
	manual := false
	for i, f := range fans {
		rpm := fmt.Sprintf(" %d/%d", f.ActualRPM, f.MaxRPM)
		if !f.TachReadable {
			rpm = " n/d"
		}
		barW := max(width-3-len(rpm), 0)
		fill := 0
		if f.TachReadable && f.MaxRPM > 0 {
			fill = min(barW*f.ActualRPM/f.MaxRPM, barW)
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "F%d %s%s%s", i, strings.Repeat("█", fill), strings.Repeat("░", barW-fill), rpm)
		manual = manual || f.Mode == 1
	}
	if manual {
		return "Ventole · BOOST (manuale)", b.String()
	}
	return "Ventole · AUTO (curva Apple)", b.String()
}

// Massimi per gruppo con le stesse chiavi di fanboost; SSD dalla temperatura NVMe SMART.
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
	return "Temperature (max)", fmt.Sprintf("CPU %-6s GPU %-6s MEM %s\nSSD %-6s BAT %s", t(cpu), t(gpu), t(mem), t(ssd), t(bat))
}
