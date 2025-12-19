module dicom-transcoder-example

go 1.25.0

require (
	github.com/cocosip/go-dicom v0.0.0-20251219052848-6a4d2da5f9d3
	github.com/cocosip/go-dicom-codec v0.0.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/text v0.30.0 // indirect
)

replace github.com/cocosip/go-dicom-codec => ../..
