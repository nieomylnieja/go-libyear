package libyear

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/nieomylnieja/go-libyear/internal"
)

type Summary struct {
	Modules  []*internal.Module
	Main     *internal.Module
	releases bool
	versions bool
}

type Output interface {
	Send(summary Summary) error
}

type TableOutput struct{}

func (p TableOutput) Send(summary Summary) error {
	data := convertSummaryToTable(summary)
	columnWidths := make([]int, len(data[0]))
	for _, row := range data {
		for i, cell := range row {
			if len(cell) > columnWidths[i] {
				columnWidths[i] = len(cell)
			}
		}
	}
	for _, row := range data {
		for i, cell := range row {
			if i == len(row)-1 {
				fmt.Print(cell)
				break
			}
			fmt.Printf("%-*s  ", columnWidths[i], cell)
		}
		fmt.Println()
	}
	return nil
}

type CSVOutput struct{}

func (p CSVOutput) Send(summary Summary) error {
	w := csv.NewWriter(os.Stdout)
	return w.WriteAll(convertSummaryToTable(summary))
}

const timeFmt = time.DateOnly

func convertSummaryToTable(summary Summary) [][]string {
	t := [][]string{
		{"package", "version", "date", "latest", "latest_date", "libyear"},
	}
	if summary.releases {
		t[0] = append(t[0], "releases")
	}
	if summary.versions {
		t[0] = append(t[0], "versions")
	}
	addRow := func(m *internal.Module) {
		row := []string{
			m.Path,                 // 0
			"",                     // 1
			m.Time.Format(timeFmt), // 2
			"",                     // 3
			"",                     // 4
			strconv.FormatFloat(m.Libyear, 'f', 2, 64), // 5
		}
		if m.Version != nil {
			row[1] = m.Version.String()
		}
		if m.Latest != nil {
			row[3] = m.Latest.Version.String()
			row[4] = m.Latest.Time.Format(timeFmt)
		}
		if summary.releases {
			row = append(row, strconv.Itoa(m.ReleasesDiff))
		}
		if summary.versions {
			row = append(row, m.VersionsDiff.String())
		}
		t = append(t, row)
	}
	addRow(summary.Main)
	for _, module := range summary.Modules {
		addRow(module)
	}
	return t
}

type JSONOutput struct{}

type jsonSummaryModel struct {
	Module   string             `json:"module"`
	Date     string             `json:"date"`
	Libyear  float64            `json:"libyear"`
	Packages []jsonPackageModel `json:"packages"`
}

type jsonPackageModel struct {
	Package       string                `json:"package"`
	Version       string                `json:"version"`
	Date          string                `json:"date"`
	LatestVersion string                `json:"latest_version"`
	LatestDate    string                `json:"latest_date"`
	Libyear       float64               `json:"libyear"`
	Releases      *int                  `json:"releases,omitempty"`
	Versions      internal.VersionsDiff `json:"versions,omitempty"`
}

func (j JSONOutput) Send(summary Summary) error {
	model := jsonSummaryModel{
		Module:   summary.Main.Path,
		Date:     summary.Main.Time.Format(timeFmt),
		Libyear:  summary.Main.Libyear,
		Packages: make([]jsonPackageModel, 0, len(summary.Modules)),
	}
	for _, module := range summary.Modules {
		m := jsonPackageModel{
			Package:       module.Path,
			Version:       module.Version.String(),
			Date:          module.Time.Format(timeFmt),
			LatestVersion: module.Latest.Version.String(),
			LatestDate:    module.Latest.Time.Format(timeFmt),
			Libyear:       module.Libyear,
		}
		if summary.releases {
			m.Releases = ptr(module.ReleasesDiff)
		}
		if summary.versions {
			m.Versions = module.VersionsDiff
		}
		model.Packages = append(model.Packages, m)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(model)
}

func ptr[T any](v T) *T { return &v }
