package t1

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/cocosip/go-dicom-codec/jpeg2000/mqc"
)

// TestFindMQDivergence - 找出MQ encoder和decoder第一个不同步的地方
func TestFindMQDivergence(t *testing.T) {
	size := 5
	numPixels := size * size
	input := make([]int32, numPixels)

	// Full gradient
	for i := 0; i < numPixels; i++ {
		input[i] = int32(i%256) - 128
	}

	maxBitplane := CalculateMaxBitplane(input)
	numPasses := (maxBitplane + 1) * 3

	t.Logf("Testing 5x5 gradient to find first MQ divergence point")
	t.Logf("Will capture first 200 MQ operations for both encoder and decoder\n")

	// Capture encoder output
	separator := "================================================================================"
	t.Logf("\n%s", separator)
	t.Logf("=== ENCODER MQ TRACE ===")
	t.Logf("%s\n", separator)

	// Redirect stdout to capture encoder output
	oldStdout := os.Stdout
	rEnc, wEnc, _ := os.Pipe()
	os.Stdout = wEnc

	mqc.EnableEncoderDebug()
	encoder := NewT1Encoder(size, size, 0)
	encoded, err := encoder.Encode(input, numPasses, 0)
	if err != nil {
		wEnc.Close()
		os.Stdout = oldStdout
		t.Fatalf("Encoding failed: %v", err)
	}

	// Restore stdout and capture output
	wEnc.Close()
	os.Stdout = oldStdout
	encOutput := new(bytes.Buffer)
	bufio.NewReader(rEnc).WriteTo(encOutput)
	rEnc.Close()

	// Capture decoder output
	t.Logf("\n%s", separator)
	t.Logf("=== DECODER MQ TRACE ===")
	t.Logf("%s\n", separator)

	// Redirect stdout to capture decoder output
	rDec, wDec, _ := os.Pipe()
	os.Stdout = wDec

	mqc.EnableDecoderDebug()
	decoder := NewT1Decoder(size, size, 0)
	err = decoder.DecodeWithBitplane(encoded, numPasses, maxBitplane, 0)
	if err != nil {
		wDec.Close()
		os.Stdout = oldStdout
		t.Fatalf("Decoding failed: %v", err)
	}

	// Restore stdout and capture output
	wDec.Close()
	os.Stdout = oldStdout
	decOutput := new(bytes.Buffer)
	bufio.NewReader(rDec).WriteTo(decOutput)
	rDec.Close()

	// Parse MQ operations
	encOps := parseMQOperations(encOutput.String(), "MQE")
	decOps := parseMQOperations(decOutput.String(), "MQC")

	t.Logf("\n%s", separator)
	t.Logf("=== COMPARISON ===")
	t.Logf("%s\n", separator)
	t.Logf("Total encoder operations: %d", len(encOps))
	t.Logf("Total decoder operations: %d\n", len(decOps))

	// Compare operations
	minLen := len(encOps)
	if len(decOps) < minLen {
		minLen = len(decOps)
	}

	firstDivergence := -1
	for i := 0; i < minLen; i++ {
		enc := encOps[i]
		dec := decOps[i]

		// Check if contexts match
		if enc.Context != dec.Context {
			t.Logf("\n*** FIRST DIVERGENCE FOUND at operation #%d ***", i+1)
			t.Logf("Encoder context: %d", enc.Context)
			t.Logf("Decoder context: %d", dec.Context)
			t.Logf("\nEncoder operation #%d:", i+1)
			t.Logf("  %s", enc.Raw)
			t.Logf("\nDecoder operation #%d:", i+1)
			t.Logf("  %s", dec.Raw)
			firstDivergence = i
			break
		}

		// Check if bit values match (for same context)
		if enc.Bit != dec.Bit {
			t.Logf("\n*** FIRST BIT MISMATCH FOUND at operation #%d ***", i+1)
			t.Logf("Context: %d (same for both)", enc.Context)
			t.Logf("Encoder bit: %d", enc.Bit)
			t.Logf("Decoder bit: %d", dec.Bit)
			t.Logf("\nEncoder operation #%d:", i+1)
			t.Logf("  %s", enc.Raw)
			t.Logf("\nDecoder operation #%d:", i+1)
			t.Logf("  %s", dec.Raw)

			// Show previous few operations for context
			if i > 0 {
				t.Logf("\nPrevious operations for context:")
				start := i - 5
				if start < 0 {
					start = 0
				}
				for j := start; j < i; j++ {
					t.Logf("  Op #%d: ENC ctx=%d bit=%d | DEC ctx=%d bit=%d",
						j+1, encOps[j].Context, encOps[j].Bit,
						decOps[j].Context, decOps[j].Bit)
				}
			}

			firstDivergence = i
			break
		}
	}

	if firstDivergence == -1 {
		t.Logf("\nNo divergence found in first %d operations!", minLen)
		t.Logf("Both encoder and decoder use identical context sequences and produce identical bits.")
	}

	// Check final result
	decoded := decoder.GetData()
	errorCount := 0
	for i := range input {
		if decoded[i] != input[i] {
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Logf("\n\nFinal result: %d errors (%.1f%%)\n", errorCount, float64(errorCount)/float64(numPixels)*100)
	} else {
		t.Logf("\n\nFinal result: PASS - all values correct!\n")
	}
}

// MQOpDetail represents a single MQ encode/decode operation with full state
type MQOpDetail struct {
	SeqNum  int
	Bit     int
	Context int
	State   int
	MPS     int
	A       uint32
	C       uint32
	Ct      int
	Raw     string // Raw line for debugging
}

// parseMQOperations parses MQ debug output
func parseMQOperations(output string, prefix string) []MQOpDetail {
	var ops []MQOpDetail

	// Pattern: [MQE #01 BEFORE] bit=1 ctx=17 state=0 mps=0 A=0x8000 C=0x00000000 ct=12
	// Pattern: [MQC #01 BEFORE] ctx=17 state=0 mps=0 A=0x8000 C=0x007fff00 ct=5
	pattern := regexp.MustCompile(fmt.Sprintf(`\[%s #(\d+) BEFORE\] (?:bit=(\d+) )?ctx=(\d+) state=(\d+) mps=(\d+) A=0x([0-9a-f]+) C=0x([0-9a-f]+) ct=(-?\d+)`, prefix))

	scanner := bufio.NewScanner(bytes.NewBufferString(output))
	for scanner.Scan() {
		line := scanner.Text()
		matches := pattern.FindStringSubmatch(line)
		if matches != nil {
			var seqNum, bit, ctx, state, mps, ct int
			var a, c uint32

			fmt.Sscanf(matches[1], "%d", &seqNum)
			if matches[2] != "" { // Encoder has bit field
				fmt.Sscanf(matches[2], "%d", &bit)
			}
			fmt.Sscanf(matches[3], "%d", &ctx)
			fmt.Sscanf(matches[4], "%d", &state)
			fmt.Sscanf(matches[5], "%d", &mps)
			fmt.Sscanf(matches[6], "%x", &a)
			fmt.Sscanf(matches[7], "%x", &c)
			fmt.Sscanf(matches[8], "%d", &ct)

			ops = append(ops, MQOpDetail{
				SeqNum:  seqNum,
				Bit:     bit,
				Context: ctx,
				State:   state,
				MPS:     mps,
				A:       a,
				C:       c,
				Ct:      ct,
				Raw:     line,
			})
		}
	}

	return ops
}
