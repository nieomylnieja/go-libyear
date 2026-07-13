package libyear

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/guptarohit/asciigraph"
)

const (
	// MinimumHistoryChartWidth is the narrowest supported terminal chart width.
	MinimumHistoryChartWidth = 40

	defaultHistoryChartWidth  = 80
	defaultHistoryChartHeight = 12
	defaultHistoryXAxisTicks  = 5
)

// HistoryChartOutput writes historical libyear samples as a terminal chart.
type HistoryChartOutput struct {
	Width  int
	Height int
	Writer io.Writer
}

// SendHistory writes an asciigraph chart for the historical samples.
func (o HistoryChartOutput) SendHistory(history History) error {
	if len(history.Samples) == 0 {
		return nil
	}

	width := o.Width
	if width <= 0 {
		width = defaultHistoryChartWidth
	}
	if width < MinimumHistoryChartWidth {
		return fmt.Errorf("history chart width must be at least %d columns", MinimumHistoryChartWidth)
	}
	height := o.Height
	if height <= 0 {
		height = defaultHistoryChartHeight
	}

	first := history.Samples[0].Timestamp.UTC()
	last := history.Samples[len(history.Samples)-1].Timestamp.UTC()
	footer := fmt.Sprintf(
		"%s -> %s | %d samples",
		first.Format(time.DateOnly),
		last.Format(time.DateOnly),
		len(history.Samples),
	)
	if utf8.RuneCountInString(footer) > width {
		return fmt.Errorf("history chart needs at least %d columns for its footer", utf8.RuneCountInString(footer))
	}
	options := []asciigraph.Option{
		asciigraph.Height(height),
		asciigraph.Precision(2),
		asciigraph.Caption("libyear history"),
		asciigraph.YAxisValueFormatter(func(v float64) string {
			return strconv.FormatFloat(v, 'f', 2, 64)
		}),
	}
	if len(history.Samples) > 1 {
		options = append(
			options,
			asciigraph.XAxisRange(historyXAxisValue(first), historyXAxisValue(last)),
			asciigraph.XAxisTickCount(historyXAxisTickCount(len(history.Samples))),
			asciigraph.XAxisValueFormatter(formatHistoryXAxisValue),
		)
	}

	graph, err := renderHistoryGraph(history, width, options)
	if err != nil {
		return err
	}
	writer := historyOutputWriter(o.Writer)
	if _, err := fmt.Fprintln(writer, graph); err != nil {
		return err
	}
	_, err = fmt.Fprintln(writer, footer)
	return err
}

// HistoryCSVOutput writes historical libyear samples as CSV.
type HistoryCSVOutput struct {
	Writer io.Writer
}

// SendHistory writes CSV rows for the historical samples.
func (o HistoryCSVOutput) SendHistory(history History) error {
	writer := csv.NewWriter(historyOutputWriter(o.Writer))
	return writer.WriteAll(convertHistoryToCSV(history))
}

// HistoryJSONOutput writes historical libyear samples as JSON.
type HistoryJSONOutput struct {
	Writer io.Writer
}

// SendHistory writes a JSON object for the historical samples.
func (o HistoryJSONOutput) SendHistory(history History) error {
	enc := json.NewEncoder(historyOutputWriter(o.Writer))
	enc.SetIndent("", "  ")
	return enc.Encode(convertHistoryToJSON(history))
}

type historyJSONModel struct {
	Module  string                   `json:"module"`
	Samples []historyJSONSampleModel `json:"samples"`
}

type historyJSONSampleModel struct {
	Timestamp string  `json:"timestamp"`
	Date      string  `json:"date"`
	Libyear   float64 `json:"libyear"`
	Packages  int     `json:"packages"`
}

func historyOutputWriter(writer io.Writer) io.Writer {
	if writer != nil {
		return writer
	}
	return os.Stdout
}

func convertHistoryToCSV(history History) [][]string {
	rows := make([][]string, 0, len(history.Samples)+1)
	rows = append(rows, []string{"module", "timestamp", "date", "libyear", "packages"})
	for _, sample := range history.Samples {
		module, libyear, packages := historySampleValues(sample)
		rows = append(rows, []string{
			module,
			sample.Timestamp.UTC().Format(time.RFC3339),
			sample.Timestamp.UTC().Format(time.DateOnly),
			strconv.FormatFloat(libyear, 'f', 2, 64),
			strconv.Itoa(packages),
		})
	}
	return rows
}

func convertHistoryToJSON(history History) historyJSONModel {
	model := historyJSONModel{
		Module:  historyModule(history),
		Samples: make([]historyJSONSampleModel, 0, len(history.Samples)),
	}
	for _, sample := range history.Samples {
		_, libyear, packages := historySampleValues(sample)
		model.Samples = append(model.Samples, historyJSONSampleModel{
			Timestamp: sample.Timestamp.UTC().Format(time.RFC3339),
			Date:      sample.Timestamp.UTC().Format(time.DateOnly),
			Libyear:   libyear,
			Packages:  packages,
		})
	}
	return model
}

func historyModule(history History) string {
	if len(history.Samples) == 0 || history.Samples[0].Summary.Main == nil {
		return ""
	}
	return history.Samples[0].Summary.Main.Path
}

func historyLibyearValues(history History) []float64 {
	values := make([]float64, 0, len(history.Samples))
	for _, sample := range history.Samples {
		_, libyear, _ := historySampleValues(sample)
		values = append(values, libyear)
	}
	return values
}

func historyXAxisValue(timestamp time.Time) float64 {
	return float64(timestamp.Unix())
}

func historyXAxisTickCount(samples int) int {
	return max(2, min(samples, defaultHistoryXAxisTicks))
}

func formatHistoryXAxisValue(value float64) string {
	return time.Unix(int64(value), 0).UTC().Format(time.DateOnly)
}

func renderHistoryGraph(history History, width int, options []asciigraph.Option) (string, error) {
	plotWidth := width
	for {
		plotOptions := append(slices.Clone(options), asciigraph.Width(plotWidth))
		graph := asciigraph.Plot(historyLibyearValuesAtWidth(history, plotWidth), plotOptions...)
		overflow := historyGraphWidth(graph) - width
		if overflow <= 0 {
			return graph, nil
		}
		if plotWidth == 1 {
			return "", fmt.Errorf("history chart needs at least %d columns for its labels", width+overflow)
		}
		plotWidth = max(1, plotWidth-overflow)
	}
}

func historyLibyearValuesAtWidth(history History, width int) []float64 {
	values := historyLibyearValues(history)
	if len(values) <= 1 || width <= 1 {
		return values
	}

	first := history.Samples[0].Timestamp
	last := history.Samples[len(history.Samples)-1].Timestamp
	span := historyXAxisValue(last) - historyXAxisValue(first)
	if span <= 0 {
		return values
	}

	interpolated := make([]float64, width)
	segment := 0
	for i := range interpolated {
		target := historyXAxisValue(first) + float64(i)*span/float64(width-1)
		for segment+1 < len(history.Samples)-1 &&
			historyXAxisValue(history.Samples[segment+1].Timestamp) < target {
			segment++
		}

		segmentStart := historyXAxisValue(history.Samples[segment].Timestamp)
		segmentEnd := historyXAxisValue(history.Samples[segment+1].Timestamp)
		if segmentEnd <= segmentStart {
			interpolated[i] = values[segment+1]
			continue
		}
		ratio := (target - segmentStart) / (segmentEnd - segmentStart)
		interpolated[i] = values[segment] + ratio*(values[segment+1]-values[segment])
	}
	return interpolated
}

func historyGraphWidth(graph string) int {
	width := 0
	for line := range strings.SplitSeq(graph, "\n") {
		width = max(width, utf8.RuneCountInString(line))
	}
	return width
}

func historySampleValues(sample HistorySample) (module string, libyear float64, packages int) {
	if sample.Summary.Main != nil {
		module = sample.Summary.Main.Path
		libyear = sample.Summary.Main.Libyear
	}
	return module, libyear, len(sample.Summary.Modules)
}
