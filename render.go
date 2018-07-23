// Package render implements HTML page creation.
package render

import (
	"appengine"
	"appengine/user"
	"html/template"
	"net/http"
	"path"

	"g/go/models"
)

// templateDir is the directory where HTML templates are stored.
const templateDir = ".../go/templates"

// depreAudits is the list of deprecated audits
var depreAudits = []string{"ced", "con"}

// These are the templates which can be rendered.
var reportTemplate, chartTemplate, fixTemplate, ticketTemplate *template.Template

// loadTemplate returns a parsed template containing the given
// template file with the layout as the base template.  Note that the
// layout filenames are formatted in a specific manner.  This function
// will panic if the templates cannot be parsed.
func loadTemplate(layoutName, templateName string) *template.Template {
	return template.Must(
		template.ParseFiles(
			path.Join(templateDir, "_"+layoutName+"_layout.html"),
			path.Join(templateDir, templateName+".html")))
}

// init reads and compiles the templates in templateDir.
func init() {
	reportTemplate = loadTemplate("main", "auditreport")
	chartTemplate = loadTemplate("main", "auditchart")
	fixTemplate = loadTemplate("main", "fixchart")
	ticketTemplate = loadTemplate("main", "auditticket")
	http.HandleFunc("/", auditChartHandler)
	http.HandleFunc("/auditreport/", auditReportHandler)
	http.HandleFunc("/auditticket/", auditTicketHandler)
	http.HandleFunc("/fixchart/", fixChartHandler)
}

// auditChartHandler renders the AuditChart page of the site.
func auditChartHandler(w http.ResponseWriter, req *http.Request) {
	c := appengine.NewContext(req)
	stats := make(map[string][]*models.StatsRecord)

	store, err := models.NewSqlStore(c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer store.Close()

	stats, err = store.AuditStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	c.Infof("stats len: %d", len(stats))

	templateData := struct {
		AuditStats       map[string][]*models.StatsRecord
		DeprecatedAudits []string
	}{
		AuditStats:       stats,
		DeprecatedAudits: depreAudits,
	}
	if err := renderLayout(c, w, chartTemplate, templateData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// fixChartHandler renders the FixChart page of the site.
func fixChartHandler(w http.ResponseWriter, req *http.Request) {
	c := appengine.NewContext(req)
	fixStats := make(map[string][]*models.FixStatsRecord)

	store, err := models.NewSqlStore(c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer store.Close()

	fixStats, err = store.FixStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	c.Infof("fixStats len: %d", len(fixStats))

	if err := renderLayout(c, w, fixTemplate, fixStats); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// auditReportHandler renders the AuditReport page of the site.
func auditReportHandler(w http.ResponseWriter, req *http.Request) {
	c := appengine.NewContext(req)
	queryParams := req.URL.Query()

	store, err := models.NewSqlStore(c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer store.Close()
	c.Infof("connected to DB")

	snapshotSelected := queryParams.Get("snapshot")
	auditNameSelected := queryParams.Get("auditname")
	auditCodeSelected := queryParams.Get("auditcode")

	snapshots, minDate, maxDate, err := store.Snapshots()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	if snapshotSelected == "" && len(snapshots) > 0 {
		snapshotSelected = snapshots[0]
	}

	var auditNames []string
	var auditRecords []*models.AuditRecord
	var auditCount map[string]int
	if len(snapshots) > 0 {
		auditCount, err = store.AuditCount(snapshotSelected)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		for k := range auditCount {
			auditNames = append(auditNames, k)
		}
		if auditNameSelected == "" && len(auditNames) > 0 {
			auditNameSelected = auditNames[0]
		}
		auditRecords, err = store.AuditRecords(snapshotSelected, auditNameSelected)
		if err != nil {
			c.Errorf("GetAuditRecords error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	templateData := struct {
		AuditRecords      []*models.AuditRecord
		AuditNames        []string
		AuditNameSelected string
		AuditCodeSelected string
		AuditCount        map[string]int
		MinDate           string
		MaxDate           string
		Snapshots         []string
		SnapshotSelected  string
	}{
		AuditRecords:      auditRecords,
		AuditNames:        auditNames,
		AuditNameSelected: auditNameSelected,
		AuditCodeSelected: auditCodeSelected,
		AuditCount:        auditCount,
		MinDate:           minDate,
		MaxDate:           maxDate,
		Snapshots:         snapshots,
		SnapshotSelected:  snapshotSelected,
	}
	if err := renderLayout(c, w, reportTemplate, templateData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// auditTicketHandler renders the ticket page of the site.
func auditTicketHandler(w http.ResponseWriter, req *http.Request) {
	c := appengine.NewContext(req)
	store, err := models.NewSqlStore(c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer store.Close()
	c.Infof("connected to DB")

	var auditTickets []*models.TicketRecord
	auditTickets, err = store.AuditTickets()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	c.Infof("tickets len: %d", len(auditTickets))

	if err := renderLayout(c, w, ticketTemplate, auditTickets); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// renderLayout renders the template with the data using the main layout.
func renderLayout(c appengine.Context, w http.ResponseWriter, template *template.Template, data interface{}) error {

	var userName string
	if u := user.Current(c); u != nil {
		userName = u.String()
	}
	headerInfo := struct {
		UserName    string
		ContentData interface{}
	}{
		UserName:    userName,
		ContentData: data,
	}

	return template.ExecuteTemplate(w, "layout", headerInfo)
}
