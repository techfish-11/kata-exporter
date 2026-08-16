package install

import _ "embed"

//go:embed assets/dashboard.json
var dashboard string

//go:embed assets/prometheus-scrape.yml
var prometheusScrape string

func DashboardJSON() string { return dashboard }

