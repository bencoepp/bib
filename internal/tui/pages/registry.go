package pages

import (
	"bib/internal/tui/app"
)

// AllPages returns all available pages for the TUI.
func AllPages(application *app.App) []app.Page {
	return []app.Page{
		NewDashboardPage(application),
		NewJobsPage(application),
		NewDatasetsPage(application),
		NewTopicsPage(application),
		NewClusterPage(application),
		NewNetworkPage(application),
		NewLogsPage(application),
		NewSettingsPage(application),
	}
}

// PageIDs contains the IDs of all pages.
const (
	PageDashboard = "dashboard"
	PageJobs      = "jobs"
	PageDatasets  = "datasets"
	PageTopics    = "topics"
	PageCluster   = "cluster"
	PageNetwork   = "network"
	PageLogs      = "logs"
	PageSettings  = "settings"
)
