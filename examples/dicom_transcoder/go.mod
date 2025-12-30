module dicom-transcoder-example

go 1.25.0

require (
	github.com/cocosip/go-dicom v0.1.8
	github.com/cocosip/go-dicom-codec v0.0.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/text v0.32.0 // indirect
)

replace github.com/cocosip/go-dicom-codec => ../..
