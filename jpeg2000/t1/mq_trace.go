package t1

import (
	"fmt"
	"sync"
)

// MQTraceEntry records a single MQ operation
type MQTraceEntry struct {
	OpType  string // "encode" or "decode"
	Context int
	Value   int
	SeqNum  int
}

// Global trace buffers
var (
	mqEncodeTrace []MQTraceEntry
	mqDecodeTrace []MQTraceEntry
	mqTraceMutex  sync.Mutex
	mqTraceEnabled bool
	mqSeqNum      int
)

// EnableMQTrace enables MQ operation tracing
func EnableMQTrace() {
	mqTraceMutex.Lock()
	defer mqTraceMutex.Unlock()
	mqTraceEnabled = true
	mqEncodeTrace = make([]MQTraceEntry, 0, 10000)
	mqDecodeTrace = make([]MQTraceEntry, 0, 10000)
	mqSeqNum = 0
}

// DisableMQTrace disables tracing
func DisableMQTrace() {
	mqTraceMutex.Lock()
	defer mqTraceMutex.Unlock()
	mqTraceEnabled = false
}

// RecordMQEncode records an encode operation
func RecordMQEncode(context, value int) {
	mqTraceMutex.Lock()
	defer mqTraceMutex.Unlock()
	if !mqTraceEnabled {
		return
	}
	mqSeqNum++
	mqEncodeTrace = append(mqEncodeTrace, MQTraceEntry{
		OpType:  "encode",
		Context: context,
		Value:   value,
		SeqNum:  mqSeqNum,
	})
}

// RecordMQDecode records a decode operation
func RecordMQDecode(context, value int) {
	mqTraceMutex.Lock()
	defer mqTraceMutex.Unlock()
	if !mqTraceEnabled {
		return
	}
	mqSeqNum++
	mqDecodeTrace = append(mqDecodeTrace, MQTraceEntry{
		OpType:  "decode",
		Context: context,
		Value:   value,
		SeqNum:  mqSeqNum,
	})
}

// CompareMQTraces compares encode and decode traces and returns the first mismatch
func CompareMQTraces() (mismatchIdx int, encEntry, decEntry *MQTraceEntry) {
	mqTraceMutex.Lock()
	defer mqTraceMutex.Unlock()

	minLen := len(mqEncodeTrace)
	if len(mqDecodeTrace) < minLen {
		minLen = len(mqDecodeTrace)
	}

	for i := 0; i < minLen; i++ {
		enc := &mqEncodeTrace[i]
		dec := &mqDecodeTrace[i]

		// Context should always match
		if enc.Context != dec.Context {
			return i, enc, dec
		}

		// Value might differ due to MQ state divergence
		// But if context matches and value differs, that's the symptom
		if enc.Value != dec.Value {
			return i, enc, dec
		}
	}

	// Check if lengths differ
	if len(mqEncodeTrace) != len(mqDecodeTrace) {
		fmt.Printf("Trace length mismatch: encode=%d, decode=%d\n",
			len(mqEncodeTrace), len(mqDecodeTrace))
	}

	return -1, nil, nil
}

// GetMQTraceStats returns statistics about the traces
func GetMQTraceStats() (encLen, decLen int) {
	mqTraceMutex.Lock()
	defer mqTraceMutex.Unlock()
	return len(mqEncodeTrace), len(mqDecodeTrace)
}
