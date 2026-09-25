// Package dashboard serves the HTML status page.
package dashboard

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/anis-mahsoume/endpoint-health-monitor/internal/checker"
	"github.com/anis-mahsoume/endpoint-health-monitor/internal/store"
)

//go:embed dashboard.html
var files embed.FS

var page = template.Must(template.ParseFS(files, "dashboard.html"))

// certWarnDays is how close to expiry a certificate gets highlighted.
const certWarnDays = 14

type row struct {
	ID     string
	URL    string
	Status string

	// Empty until the first check has run.
	LastChecked string
	Latency     string

	// Days until the TLS certificate expires, e.g. "42 days".
	// Empty for plain HTTP or when the last check failed.
	CertExpiry string
	CertSoon   bool

	// Chip hover text: the status (or "Down since ...") plus the last error.
	Tooltip string
}

type pageData struct {
	GeneratedAt time.Time
	Endpoints   []row
}

// Register mounts the dashboard page at the root of rg.
func Register(rg gin.IRouter, st *store.Store) {
	rg.GET("/", func(c *gin.Context) {
		render(c, st)
	})
}

func render(c *gin.Context, st *store.Store) {
	states := st.Snapshot()

	data := pageData{
		GeneratedAt: time.Now(),
		Endpoints:   make([]row, 0, len(states)),
	}
	for _, s := range states {
		r := row{
			ID:     s.Endpoint.ID,
			URL:    s.Endpoint.URL,
			Status: s.Status(),
		}
		var errMsg string
		if latest, ok := s.Latest(); ok {
			r.LastChecked = latest.CheckedAt.Format("15:04:05")
			r.Latency = latest.Latency.Round(time.Millisecond).String()
			errMsg = errorText(latest)
			if latest.CertExpiry != nil {
				days := int(time.Until(*latest.CertExpiry).Hours() / 24)
				r.CertExpiry = daysText(days)
				r.CertSoon = days < certWarnDays
			}
		}

		r.Tooltip = strings.ToUpper(r.Status[:1]) + r.Status[1:]
		if s.Down && s.FailingSince != nil {
			r.Tooltip = "Down since " + s.FailingSince.Format("15:04:05")
		}
		if errMsg != "" {
			r.Tooltip += "\n" + errMsg
		}

		data.Endpoints = append(data.Endpoints, r)
	}

	var buf bytes.Buffer
	if err := page.Execute(&buf, data); err != nil {
		log.Printf("dashboard: render failed: %v", err)
		c.String(http.StatusInternalServerError, "could not render dashboard")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

// errorText describes why a check failed, or returns "" if it was up.
func errorText(r checker.Result) string {
	switch {
	case r.Outcome == checker.OutcomeUp:
		return ""
	case r.Err != "":
		return r.Err
	case r.Outcome == checker.OutcomeBadStatus:
		return fmt.Sprintf("HTTP %d", r.StatusCode)
	default:
		return string(r.Outcome)
	}
}

// daysText formats a day count as "1 day", "42 days" or "expired".
func daysText(days int) string {
	switch {
	case days < 0:
		return "expired"
	case days == 1:
		return "1 day"
	default:
		return fmt.Sprintf("%d days", days)
	}
}
