bundle-eb:
		zip go.zip -r api util app.env application.go go.mod go.sum

.PHONY: bundle-eb
