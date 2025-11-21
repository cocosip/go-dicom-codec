package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
)

type MQOp struct {
	num   int
	bit   int // -1 for decoder (unknown before AFTER)
	ctx   int
	state int
	mps   int
	A     int
	C     int
	ct    int
}

func main() {
	file, err := os.Open("mq_debug_output.txt")
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	encPattern := regexp.MustCompile(`\[MQE #(\d+) BEFORE\] bit=(\d+) ctx=(\d+) state=(\d+) mps=(\d+) A=0x([0-9a-f]+) C=0x([0-9a-f]+) ct=(-?\d+)`)
	decPattern := regexp.MustCompile(`\[MQC #(\d+) BEFORE\] ctx=(\d+) state=(\d+) mps=(\d+) A=0x([0-9a-f]+) C=0x([0-9a-f]+) ct=(-?\d+)`)
	decAfterPattern := regexp.MustCompile(`\[MQC #(\d+) AFTER \] bit=(\d+)`)

	encOps := make(map[int]*MQOp)
	decOps := make(map[int]*MQOp)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Parse encoder operations
		if match := encPattern.FindStringSubmatch(line); match != nil {
			num, _ := strconv.Atoi(match[1])
			bit, _ := strconv.Atoi(match[2])
			ctx, _ := strconv.Atoi(match[3])
			state, _ := strconv.Atoi(match[4])
			mps, _ := strconv.Atoi(match[5])
			A, _ := strconv.ParseInt(match[6], 16, 64)
			C, _ := strconv.ParseInt(match[7], 16, 64)
			ct, _ := strconv.Atoi(match[8])

			encOps[num] = &MQOp{
				num: num, bit: bit, ctx: ctx, state: state,
				mps: mps, A: int(A), C: int(C), ct: ct,
			}
		}

		// Parse decoder BEFORE operations
		if match := decPattern.FindStringSubmatch(line); match != nil {
			num, _ := strconv.Atoi(match[1])
			ctx, _ := strconv.Atoi(match[2])
			state, _ := strconv.Atoi(match[3])
			mps, _ := strconv.Atoi(match[4])
			A, _ := strconv.ParseInt(match[5], 16, 64)
			C, _ := strconv.ParseInt(match[6], 16, 64)
			ct, _ := strconv.Atoi(match[7])

			decOps[num] = &MQOp{
				num: num, bit: -1, ctx: ctx, state: state,
				mps: mps, A: int(A), C: int(C), ct: ct,
			}
		}

		// Parse decoder AFTER operations to get bit
		if match := decAfterPattern.FindStringSubmatch(line); match != nil {
			num, _ := strconv.Atoi(match[1])
			bit, _ := strconv.Atoi(match[2])
			if op, exists := decOps[num]; exists {
				op.bit = bit
			}
		}
	}

	fmt.Printf("Parsed %d encoder operations\n", len(encOps))
	fmt.Printf("Parsed %d decoder operations\n\n", len(decOps))

	// Compare operations 1-200
	firstMismatch := -1
	for i := 1; i <= 200; i++ {
		enc, encOK := encOps[i]
		dec, decOK := decOps[i]

		if !encOK || !decOK {
			continue
		}

		// Check context mismatch
		if enc.ctx != dec.ctx {
			fmt.Printf("*** CONTEXT MISMATCH at op #%d ***\n", i)
			fmt.Printf("  ENC: ctx=%d state=%d mps=%d A=0x%04x bit=%d\n", enc.ctx, enc.state, enc.mps, enc.A, enc.bit)
			fmt.Printf("  DEC: ctx=%d state=%d mps=%d A=0x%04x\n", dec.ctx, dec.state, dec.mps, dec.A)
			firstMismatch = i
			break
		}

		// Check state mismatch (same context)
		if enc.state != dec.state {
			fmt.Printf("*** STATE MISMATCH at op #%d (same context) ***\n", i)
			fmt.Printf("  Context: %d\n", enc.ctx)
			fmt.Printf("  ENC: state=%d mps=%d A=0x%04x\n", enc.state, enc.mps, enc.A)
			fmt.Printf("  DEC: state=%d mps=%d A=0x%04x\n", dec.state, dec.mps, dec.A)
			firstMismatch = i
			break
		}

		// Check A register mismatch (same context, same state)
		if enc.A != dec.A {
			fmt.Printf("*** A REGISTER MISMATCH at op #%d (same context, same state) ***\n", i)
			fmt.Printf("  Context: %d, State: %d\n", enc.ctx, enc.state)
			fmt.Printf("  ENC: A=0x%04x\n", enc.A)
			fmt.Printf("  DEC: A=0x%04x\n", dec.A)
			firstMismatch = i
			break
		}

		// Check bit mismatch (everything matches)
		if dec.bit >= 0 && enc.bit != dec.bit {
			fmt.Printf("*** BIT MISMATCH at op #%d (same ctx=%d, state=%d, A=0x%04x) ***\n",
				i, enc.ctx, enc.state, enc.A)
			fmt.Printf("  ENC bit: %d\n", enc.bit)
			fmt.Printf("  DEC bit: %d\n", dec.bit)
			fmt.Printf("  This means decoder read wrong bit from bitstream!\n")
			firstMismatch = i
			break
		}
	}

	if firstMismatch == -1 {
		fmt.Println("All operations 1-200: contexts, states, A registers, and bits MATCH!")
	} else {
		fmt.Printf("\nFirst divergence at operation #%d\n", firstMismatch)
	}
}
