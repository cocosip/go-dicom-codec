package main

import (
	"testing"

	"github.com/cocosip/go-dicom/pkg/dicom/dataset"
	"github.com/cocosip/go-dicom/pkg/dicom/element"
	"github.com/cocosip/go-dicom/pkg/dicom/tag"
	"github.com/cocosip/go-dicom/pkg/dicom/transfer"
	"github.com/cocosip/go-dicom/pkg/dicom/vr"
)

func TestApplyBaselinePhotometricInterpretationMatchesFoDicom(t *testing.T) {
	ds := dataset.New()
	if err := ds.Add(element.NewUnsignedShort(tag.SamplesPerPixel, []uint16{3})); err != nil {
		t.Fatal(err)
	}
	if err := ds.Add(element.NewString(tag.PhotometricInterpretation, vr.CS, []string{"RGB"})); err != nil {
		t.Fatal(err)
	}

	if err := applyBaselinePhotometricInterpretation(ds, transfer.JPEGBaseline8Bit); err != nil {
		t.Fatalf("applyBaselinePhotometricInterpretation() error = %v", err)
	}
	if got := ds.TryGetString(tag.PhotometricInterpretation); got != photometricInterpretationYBRFull422 {
		t.Errorf("PhotometricInterpretation = %q, want %s", got, photometricInterpretationYBRFull422)
	}
}

func TestApplyLossyImageCompressionMetadataMatchesTransferSyntax(t *testing.T) {
	ds := dataset.New()
	if err := applyLossyImageCompressionMetadata(ds, transfer.JPEGBaseline8Bit, 1243, 200); err != nil {
		t.Fatalf("applyLossyImageCompressionMetadata() error = %v", err)
	}

	if got := ds.TryGetString(tag.LossyImageCompression); got != "01" {
		t.Errorf("LossyImageCompression = %q, want 01", got)
	}
	if got := ds.TryGetString(tag.LossyImageCompressionRatio); got != "6.215" {
		t.Errorf("LossyImageCompressionRatio = %q, want 6.215", got)
	}
	if got := ds.TryGetString(tag.LossyImageCompressionMethod); got != "ISO_10918_1" {
		t.Errorf("LossyImageCompressionMethod = %q, want ISO_10918_1", got)
	}
}
