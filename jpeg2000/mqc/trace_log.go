package mqc

import (
	"fmt"
	"os"
	"sync"
)

var (
	encLog     *os.File
	decLog     *os.File
	logMutex   sync.Mutex
	loggingEnabled bool
)

// EnableMQLogging enables logging of all MQ operations to files
func EnableMQLogging() error {
	logMutex.Lock()
	defer logMutex.Unlock()

	var err error
	encLog, err = os.Create("mq_encoder.log")
	if err != nil {
		return err
	}

	decLog, err = os.Create("mq_decoder.log")
	if err != nil {
		encLog.Close()
		return err
	}

	loggingEnabled = true
	return nil
}

// DisableMQLogging disables logging and closes log files
func DisableMQLogging() {
	logMutex.Lock()
	defer logMutex.Unlock()

	if encLog != nil {
		encLog.Close()
		encLog = nil
	}
	if decLog != nil {
		decLog.Close()
		decLog = nil
	}
	loggingEnabled = false
}

func logEncoderOp(opNum int, bit, ctx int, stateBefore, stateAfter uint8, aBefore, aAfter, cBefore, cAfter uint32) {
	if !loggingEnabled || encLog == nil {
		return
	}

	logMutex.Lock()
	defer logMutex.Unlock()

	fmt.Fprintf(encLog, "%05d ctx=%02d bit=%d state=%02d->%02d mps=%d->%d A=%04x->%04x C=%08x->%08x\n",
		opNum, ctx, bit,
		stateBefore&0x7F, stateAfter&0x7F,
		int(stateBefore>>7), int(stateAfter>>7),
		aBefore, aAfter,
		cBefore, cAfter)
}

func logDecoderOp(opNum int, bit, ctx int, stateBefore, stateAfter uint8, aBefore, aAfter, cBefore, cAfter uint32) {
	if !loggingEnabled || decLog == nil {
		return
	}

	logMutex.Lock()
	defer logMutex.Unlock()

	fmt.Fprintf(decLog, "%05d ctx=%02d bit=%d state=%02d->%02d mps=%d->%d A=%04x->%04x C=%08x->%08x\n",
		opNum, ctx, bit,
		stateBefore&0x7F, stateAfter&0x7F,
		int(stateBefore>>7), int(stateAfter>>7),
		aBefore, aAfter,
		cBefore, cAfter)
}
