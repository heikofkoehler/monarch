// Package portfolio provides data structures and output utilities for Monarch Money portfolio data.
package portfolio

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// --- JSON data structures ---

type Response struct {
	Portfolio PortfolioData `json:"portfolio"`
}

type PortfolioData struct {
	AggregateHoldings AggregateHoldings `json:"aggregateHoldings"`
}

type AggregateHoldings struct {
	Edges []Edge `json:"edges"`
}

type Edge struct {
	Node AggregateNode `json:"node"`
}

type AggregateNode struct {
	Security Security  `json:"security"`
	Holdings []Holding `json:"holdings"`
}

type Security struct {
	ID                    string  `json:"id"`
	Name                  string  `json:"name"`
	Ticker                string  `json:"ticker"`
	CurrentPrice          float64 `json:"currentPrice"`
	CurrentPriceUpdatedAt string  `json:"currentPriceUpdatedAt"`
	Type                  string  `json:"type"`
	TypeDisplay           string  `json:"typeDisplay"`
}

type Holding struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	TypeDisplay  string  `json:"typeDisplay"`
	Name         string  `json:"name"`
	Ticker       string  `json:"ticker"`
	ClosingPrice float64 `json:"closingPrice"`
	Quantity     float64 `json:"quantity"`
	Value        float64 `json:"value"`
	Account      Account `json:"account"`
}

type Account struct {
	ID          string      `json:"id"`
	Mask        string      `json:"mask"`
	DisplayName string      `json:"displayName"`
	Institution Institution `json:"institution"`
}

type Institution struct {
	Name string `json:"name"`
}

// --- Extracted flat record ---

type HoldingRecord struct {
	AccountID       string
	AccountName     string
	AccountMask     string
	InstitutionName string
	HoldingName     string
	Ticker          string
	Type            string
	TypeDisplay     string
	Quantity        float64
	ClosingPrice    float64
	Value           float64
	SecurityID      string
	SecurityName    string
	SecurityTicker  string
	CurrentPrice    float64
	PriceUpdated    string
}

var csvHeaders = []string{
	"account_id", "account_name", "account_mask", "institution_name",
	"holding_name", "ticker", "type", "type_display",
	"quantity", "closing_price", "value",
	"security_id", "security_name", "security_ticker",
	"current_price", "price_updated",
}

func (r HoldingRecord) toRow() []string {
	return []string{
		r.AccountID,
		r.AccountName,
		r.AccountMask,
		r.InstitutionName,
		r.HoldingName,
		r.Ticker,
		r.Type,
		r.TypeDisplay,
		fmt.Sprintf("%g", r.Quantity),
		fmt.Sprintf("%g", r.ClosingPrice),
		fmt.Sprintf("%g", r.Value),
		r.SecurityID,
		r.SecurityName,
		r.SecurityTicker,
		fmt.Sprintf("%g", r.CurrentPrice),
		r.PriceUpdated,
	}
}

// ExtractHoldings parses a portfolio response and returns a flat list of holding records
// sorted by value descending.
func ExtractHoldings(resp *Response) []HoldingRecord {
	var records []HoldingRecord
	for _, edge := range resp.Portfolio.AggregateHoldings.Edges {
		sec := edge.Node.Security
		for _, h := range edge.Node.Holdings {
			records = append(records, HoldingRecord{
				AccountID:       h.Account.ID,
				AccountName:     h.Account.DisplayName,
				AccountMask:     h.Account.Mask,
				InstitutionName: h.Account.Institution.Name,
				HoldingName:     h.Name,
				Ticker:          h.Ticker,
				Type:            h.Type,
				TypeDisplay:     h.TypeDisplay,
				Quantity:        h.Quantity,
				ClosingPrice:    h.ClosingPrice,
				Value:           h.Value,
				SecurityID:      sec.ID,
				SecurityName:    sec.Name,
				SecurityTicker:  sec.Ticker,
				CurrentPrice:    sec.CurrentPrice,
				PriceUpdated:    sec.CurrentPriceUpdatedAt,
			})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Value > records[j].Value
	})
	return records
}

// LoadResponse reads and parses a portfolio JSON file.
func LoadResponse(path string) (*Response, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var resp Response
	if err := json.NewDecoder(f).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return &resp, nil
}

// WriteCSV writes holding records to a CSV file.
func WriteCSV(records []HoldingRecord, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(csvHeaders); err != nil {
		return err
	}
	for _, r := range records {
		if err := w.Write(r.toRow()); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// WriteMarkdown writes holding records as a Markdown table to w.
func WriteMarkdown(records []HoldingRecord, w io.Writer) {
	colWidths := make([]int, len(csvHeaders))
	for i, h := range csvHeaders {
		colWidths[i] = len(h)
	}
	rows := make([][]string, len(records))
	for i, r := range records {
		row := r.toRow()
		rows[i] = row
		for j, cell := range row {
			if len(cell) > colWidths[j] {
				colWidths[j] = len(cell)
			}
		}
	}

	printRow := func(cells []string) {
		fmt.Fprint(w, "|")
		for i, cell := range cells {
			fmt.Fprintf(w, " %-*s |", colWidths[i], cell)
		}
		fmt.Fprintln(w)
	}

	printRow(csvHeaders)

	fmt.Fprint(w, "|")
	for _, width := range colWidths {
		fmt.Fprintf(w, " %s |", strings.Repeat("-", width))
	}
	fmt.Fprintln(w)

	for _, row := range rows {
		printRow(row)
	}
}
