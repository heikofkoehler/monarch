package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/heikofkoehler/monarch/internal/client"
	"github.com/heikofkoehler/monarch/internal/portfolio"
)

const portfolioQuery = `query Web_GetPortfolio($portfolioInput: PortfolioInput) {
  portfolio(input: $portfolioInput) {
    aggregateHoldings {
      edges {
        node {
          holdings {
            id
            type
            typeDisplay
            name
            ticker
            closingPrice
            closingPriceUpdatedAt
            quantity
            value
            account {
              id
              mask
              displayName
              institution {
                id
                name
                __typename
              }
              __typename
            }
            __typename
          }
          security {
            id
            name
            ticker
            currentPrice
            currentPriceUpdatedAt
            closingPrice
            type
            typeDisplay
            __typename
          }
          __typename
        }
        __typename
      }
      __typename
    }
    __typename
  }
}`

// credentials loaded from a JSON file or environment variables.
type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func loadCredentials(path string) (credentials, error) {
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		var c credentials
		if err := json.NewDecoder(f).Decode(&c); err != nil {
			return credentials{}, fmt.Errorf("parse %s: %w", path, err)
		}
		if c.Email != "" && c.Password != "" {
			return c, nil
		}
	}

	// Fall back to environment variables.
	c := credentials{
		Email:    os.Getenv("MONARCH_EMAIL"),
		Password: os.Getenv("MONARCH_PASSWORD"),
	}
	if c.Email == "" || c.Password == "" {
		return credentials{}, fmt.Errorf(
			"credentials not found: create %s with {\"email\":...,\"password\":...} or set MONARCH_EMAIL and MONARCH_PASSWORD",
			path,
		)
	}
	return c, nil
}

func prompt(label string) string {
	fmt.Fprint(os.Stdout, label)
	sc := bufio.NewScanner(os.Stdin)
	sc.Scan()
	return strings.TrimSpace(sc.Text())
}

// authenticate logs in to Monarch Money, handling MFA interactively.
// It tries a saved session first, then falls back to email/password.
func authenticate(c *client.Client, credsPath string, useSavedSession bool) error {
	if useSavedSession {
		loaded, err := c.LoadSession()
		if err != nil {
			return fmt.Errorf("load session: %w", err)
		}
		if loaded {
			fmt.Println("Using saved session.")
			return nil
		}
	}

	creds, err := loadCredentials(credsPath)
	if err != nil {
		return err
	}

	err = c.Login(creds.Email, creds.Password, "")
	if err == nil {
		return c.SaveSession()
	}
	if !errors.Is(err, client.ErrMFARequired) {
		return fmt.Errorf("login failed: %w", err)
	}

	// MFA required — prompt user.
	fmt.Println("Multi-factor authentication required.")
	code := prompt("Two-factor code: ")
	if err := c.Login(creds.Email, creds.Password, code); err != nil {
		return fmt.Errorf("MFA login failed: %w", err)
	}
	return c.SaveSession()
}

// fetchPortfolio fetches the portfolio from the Monarch API and returns the raw JSON.
func fetchPortfolio(c *client.Client) (json.RawMessage, error) {
	data, err := c.GraphQLCall("Web_GetPortfolio", portfolioQuery, map[string]any{})
	if err != nil {
		return nil, err
	}
	raw, ok := data["portfolio"]
	if !ok {
		return nil, fmt.Errorf("portfolio key missing from GraphQL response")
	}
	// Wrap it back in the expected {"portfolio": ...} envelope.
	wrapped, err := json.Marshal(map[string]json.RawMessage{"portfolio": raw})
	if err != nil {
		return nil, err
	}
	return wrapped, nil
}

// ---- subcommands ----

func cmdFetch(args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)
	credsPath := fs.String("c", "credentials.json", "Path to credentials JSON file")
	outFile := fs.String("o", "portfolio.json", "Output JSON filename")
	csvFile := fs.String("csv", "", "Output CSV filename for holdings (optional)")
	noSession := fs.Bool("no-session", false, "Skip saved session and always re-authenticate")
	token := fs.String("token", "", "Auth token (skips login; use token from browser DevTools)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: monarch fetch [options]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	c := client.New()
	if *token != "" {
		c.SetToken(*token)
	} else if err := authenticate(c, *credsPath, !*noSession); err != nil {
		return err
	}

	raw, err := fetchPortfolio(c)
	if err != nil {
		return fmt.Errorf("fetch portfolio: %w", err)
	}

	// Pretty-print JSON to file.
	var pretty interface{}
	if err := json.Unmarshal(raw, &pretty); err != nil {
		return err
	}
	f, err := os.Create(*outFile)
	if err != nil {
		return fmt.Errorf("create %s: %w", *outFile, err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	if err := enc.Encode(pretty); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Printf("Saved portfolio to %s\n", *outFile)

	if *csvFile != "" {
		resp, err := portfolio.LoadResponse(*outFile)
		if err != nil {
			return err
		}
		records := portfolio.ExtractHoldings(resp)
		if err := portfolio.WriteCSV(records, *csvFile); err != nil {
			return fmt.Errorf("write CSV: %w", err)
		}
		fmt.Printf("Wrote %d holdings to %s\n", len(records), *csvFile)
	}

	fmt.Println("Sync complete!")
	return nil
}

func cmdParse(args []string) error {
	fs := flag.NewFlagSet("parse", flag.ExitOnError)
	inFile := fs.String("i", "portfolio.json", "Input JSON portfolio file")
	outFile := fs.String("o", "portfolio_holdings.csv", "Output CSV filename")
	markdown := fs.Bool("markdown", false, "Display output as markdown table")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: monarch parse [options]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	resp, err := portfolio.LoadResponse(*inFile)
	if err != nil {
		return err
	}
	records := portfolio.ExtractHoldings(resp)

	if *markdown {
		portfolio.WriteMarkdown(records, os.Stdout)
	}

	if err := portfolio.WriteCSV(records, *outFile); err != nil {
		return fmt.Errorf("write CSV: %w", err)
	}
	fmt.Printf("Saved %d holdings to %s\n", len(records), *outFile)
	return nil
}

func cmdPipeline(args []string) error {
	fs := flag.NewFlagSet("pipeline", flag.ExitOnError)
	credsPath := fs.String("c", "credentials.json", "Path to credentials JSON file")
	portfolioJSON := fs.String("portfolio-json", "portfolio.json", "Intermediate portfolio JSON file")
	portfolioCSV := fs.String("portfolio-csv", "portfolio_holdings.csv", "Output CSV file")
	skipFetch := fs.Bool("skip-fetch", false, "Skip fetching, only parse existing JSON")
	noSession := fs.Bool("no-session", false, "Skip saved session and always re-authenticate")
	token := fs.String("token", "", "Auth token (skips login; use token from browser DevTools)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: monarch pipeline [options]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*skipFetch {
		fmt.Println("\n=== Step 1: Fetching portfolio from Monarch Money ===")
		fetchArgs := []string{"-c", *credsPath, "-o", *portfolioJSON}
		if *noSession {
			fetchArgs = append(fetchArgs, "-no-session")
		}
		if *token != "" {
			fetchArgs = append(fetchArgs, "-token", *token)
		}
		if err := cmdFetch(fetchArgs); err != nil {
			return fmt.Errorf("fetch step: %w", err)
		}
	}

	fmt.Println("\n=== Step 2: Parsing portfolio to CSV ===")
	if err := cmdParse([]string{"-i", *portfolioJSON, "-o", *portfolioCSV}); err != nil {
		return fmt.Errorf("parse step: %w", err)
	}

	fmt.Println("\n=== Pipeline completed successfully ===")
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, `Monarch Money portfolio tools

Usage:
  monarch <command> [options]

Commands:
  fetch     Fetch portfolio from Monarch Money API and save to JSON
  parse     Parse portfolio JSON and export to CSV (and optionally Markdown)
  pipeline  Run fetch then parse in sequence

Run "monarch <command> -h" for command-specific options.`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "fetch":
		err = cmdFetch(os.Args[2:])
	case "parse":
		err = cmdParse(os.Args[2:])
	case "pipeline":
		err = cmdPipeline(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
