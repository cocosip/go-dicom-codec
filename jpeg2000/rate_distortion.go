package jpeg2000

import (
	"math"
	"sort"

	"github.com/cocosip/go-dicom-codec/jpeg2000/t1"
)

// LayerAllocation represents pass allocation for quality layers
type LayerAllocation struct {
	// Number of layers
	NumLayers int

	// Pass allocation for each code-block
	// [codeblock_index][layer] = number of passes included up to this layer
	// Example: if CodeBlockPasses[0] = [3, 7, 10], then:
	//   - Layer 0: includes passes 0-2 (3 passes)
	//   - Layer 1: includes passes 0-6 (7 passes total, 4 new)
	//   - Layer 2: includes passes 0-9 (10 passes total, 3 new)
	CodeBlockPasses [][]int
}

// CodeBlockContribution represents rate-distortion contribution of a code-block pass
type CodeBlockContribution struct {
	CodeBlockIndex int     // Index of the code-block
	PassIndex      int     // Index of the pass (0-based)
	Rate           float64 // Size in bytes
	Distortion     float64 // Distortion reduction (lower is better)
	Slope          float64 // Rate-distortion slope (distortion/rate)
}

// AllocateLayersSimple performs simple layer allocation based on pass distribution
// This is a simplified version that doesn't require full PCRD-opt
//
// Strategy:
// - Divide total passes evenly across layers
// - Layer 0 gets first 1/N of passes
// - Layer 1 gets first 2/N of passes
// - Layer N-1 gets all passes
func AllocateLayersSimple(totalPasses int, numLayers int, numCodeBlocks int) *LayerAllocation {
	if numLayers <= 0 {
		numLayers = 1
	}

	if numLayers == 1 {
		// Single layer: include all passes
		alloc := &LayerAllocation{
			NumLayers:       1,
			CodeBlockPasses: make([][]int, numCodeBlocks),
		}
		for i := 0; i < numCodeBlocks; i++ {
			alloc.CodeBlockPasses[i] = []int{totalPasses}
		}
		return alloc
	}

	alloc := &LayerAllocation{
		NumLayers:       numLayers,
		CodeBlockPasses: make([][]int, numCodeBlocks),
	}

	// Calculate cumulative passes for each layer
	// Using geometric progression for better quality distribution
	// Layer 0: ~25% of passes
	// Layer 1: ~50% of passes
	// Layer 2: ~75% of passes
	// Layer N-1: 100% of passes

	for cb := 0; cb < numCodeBlocks; cb++ {
		layerPasses := make([]int, numLayers)

		for layer := 0; layer < numLayers; layer++ {
			if layer == numLayers-1 {
				// Last layer: all passes
				layerPasses[layer] = totalPasses
			} else {
				// Progressive allocation: use exponential distribution
				// fraction = (layer+1) / numLayers raised to power 0.7
				fraction := math.Pow(float64(layer+1)/float64(numLayers), 0.7)
				passes := int(float64(totalPasses) * fraction)

				// Ensure at least 1 pass per layer and monotonically increasing
				if passes < layer+1 {
					passes = layer + 1
				}
				if passes > totalPasses {
					passes = totalPasses
				}

				layerPasses[layer] = passes
			}
		}

		alloc.CodeBlockPasses[cb] = layerPasses
	}

	return alloc
}

// AllocateLayersRateDistortion performs rate-distortion optimized layer allocation
// This implements a simplified PCRD-opt (Post-Compression Rate-Distortion optimization)
//
// Reference: ISO/IEC 15444-1:2019 Annex J.2
func AllocateLayersRateDistortion(
	codeBlockSizes [][]int, // [codeblock][pass] = size in bytes
	targetRates []float64, // Target rates for each layer (in bytes)
) *LayerAllocation {
	numCodeBlocks := len(codeBlockSizes)
	if numCodeBlocks == 0 {
		return &LayerAllocation{NumLayers: 1}
	}

	numLayers := len(targetRates)
	if numLayers == 0 {
		numLayers = 1
		// Default: use all data for single layer
		maxSize := 0.0
		for _, cbSizes := range codeBlockSizes {
			for _, size := range cbSizes {
				maxSize += float64(size)
			}
		}
		targetRates = []float64{maxSize}
	}

	alloc := &LayerAllocation{
		NumLayers:       numLayers,
		CodeBlockPasses: make([][]int, numCodeBlocks),
	}

	// Build list of all contributions
	contributions := make([]CodeBlockContribution, 0)
	for cbIdx, cbSizes := range codeBlockSizes {
		cumulativeSize := 0.0
		for passIdx, size := range cbSizes {
			cumulativeSize += float64(size)

			// Estimate distortion reduction (simplified)
			// In reality, this would be computed from actual distortion metrics
			// Higher bit-planes contribute more to distortion reduction
			distortionReduction := math.Pow(2.0, float64(len(cbSizes)-passIdx))

			slope := 0.0
			if cumulativeSize > 0 {
				slope = distortionReduction / cumulativeSize
			}

			contributions = append(contributions, CodeBlockContribution{
				CodeBlockIndex: cbIdx,
				PassIndex:      passIdx,
				Rate:           cumulativeSize,
				Distortion:     distortionReduction,
				Slope:          slope,
			})
		}
	}

	// Sort contributions by slope (descending - best contributions first)
	sort.Slice(contributions, func(i, j int) bool {
		return contributions[i].Slope > contributions[j].Slope
	})

	// Allocate contributions to layers based on target rates
	for cb := 0; cb < numCodeBlocks; cb++ {
		alloc.CodeBlockPasses[cb] = make([]int, numLayers)
	}

	for layerIdx := 0; layerIdx < numLayers; layerIdx++ {
		targetRate := targetRates[layerIdx]
		currentRate := 0.0

		// Include contributions until we reach target rate
		for _, contrib := range contributions {
			if currentRate >= targetRate {
				break
			}

			cbIdx := contrib.CodeBlockIndex
			passIdx := contrib.PassIndex

			// Update layer allocation
			if alloc.CodeBlockPasses[cbIdx][layerIdx] <= passIdx {
				alloc.CodeBlockPasses[cbIdx][layerIdx] = passIdx + 1
			}

			currentRate = contrib.Rate
		}

		// Ensure monotonically increasing passes across layers
		if layerIdx > 0 {
			for cb := 0; cb < numCodeBlocks; cb++ {
				if alloc.CodeBlockPasses[cb][layerIdx] < alloc.CodeBlockPasses[cb][layerIdx-1] {
					alloc.CodeBlockPasses[cb][layerIdx] = alloc.CodeBlockPasses[cb][layerIdx-1]
				}
			}
		}
	}

	return alloc
}

// AllocateLayersRateDistortionPasses performs PCRD-like allocation using per-pass rate/distortion.
// passesPerBlock: [codeblock][pass] PassData with cumulative rate/distortion.
// targetBudget: total byte budget for the final layer (cumulative). If <=0, uses full rate.
func AllocateLayersRateDistortionPasses(
	passesPerBlock [][]t1.PassData,
	numLayers int,
	targetBudget float64,
) *LayerAllocation {
	numBlocks := len(passesPerBlock)
	if numBlocks == 0 {
		if numLayers <= 0 {
			numLayers = 1
		}
		return &LayerAllocation{NumLayers: numLayers}
	}
	if numLayers <= 0 {
		numLayers = 1
	}
	alloc := &LayerAllocation{
		NumLayers:       numLayers,
		CodeBlockPasses: make([][]int, numBlocks),
	}
	for i := 0; i < numBlocks; i++ {
		alloc.CodeBlockPasses[i] = make([]int, numLayers)
	}

	if numLayers == 1 {
		for cb := 0; cb < numBlocks; cb++ {
			alloc.CodeBlockPasses[cb][0] = len(passesPerBlock[cb])
		}
		return alloc
	}

	type contrib struct {
		CodeBlockIndex int
		PassIndex      int
		Rate           int
		Distortion     float64
		Slope          float64
	}

	contribs := make([]contrib, 0)
	totalRate := 0.0
	for cbIdx, passes := range passesPerBlock {
		prevRate := 0
		prevDist := 0.0
		for pi, p := range passes {
			cumRate := p.ActualBytes
			if cumRate == 0 {
				cumRate = p.Rate
			}
			incRate := cumRate - prevRate
			if incRate <= 0 {
				incRate = 1
			}
			incDist := p.Distortion - prevDist
			if incDist < 0 {
				incDist = 0
			}
			slope := 0.0
			if incRate > 0 {
				slope = incDist / float64(incRate)
			}
			contribs = append(contribs, contrib{
				CodeBlockIndex: cbIdx,
				PassIndex:      pi,
				Rate:           incRate,
				Distortion:     incDist,
				Slope:          slope,
			})
			prevRate = cumRate
			prevDist = p.Distortion
		}
		totalRate += float64(getPassBytes(passes, len(passes)))
	}

	// Clamp budget
	if targetBudget <= 0 || targetBudget > totalRate {
		targetBudget = totalRate
	}

	sort.Slice(contribs, func(i, j int) bool {
		return contribs[i].Slope > contribs[j].Slope
	})

	// Build cumulative targets per layer (progressive fraction of final budget)
	targetRates := make([]float64, numLayers)
	for layer := 0; layer < numLayers; layer++ {
		frac := math.Pow(float64(layer+1)/float64(numLayers), 1.1)
		targetRates[layer] = targetBudget * frac
	}

	selected := make([]int, numBlocks) // passes selected (cumulative) for current layer
	for layer := 0; layer < numLayers; layer++ {
		currentRate := 0.0
		// rate contributed by already selected passes (previous layers)
		for cb := 0; cb < numBlocks; cb++ {
			currentRate += float64(getPassBytes(passesPerBlock[cb], selected[cb]))
		}

		budget := targetRates[layer]
		for _, c := range contribs {
			if currentRate >= budget {
				break
			}
			if c.PassIndex+1 <= selected[c.CodeBlockIndex] {
				continue
			}
			newCount := c.PassIndex + 1
			delta := getPassBytes(passesPerBlock[c.CodeBlockIndex], newCount) - getPassBytes(passesPerBlock[c.CodeBlockIndex], selected[c.CodeBlockIndex])
			if delta <= 0 {
				continue
			}
			selected[c.CodeBlockIndex] = newCount
			currentRate += float64(delta)
		}

		// Record allocation for this layer
		for cb := 0; cb < numBlocks; cb++ {
			alloc.CodeBlockPasses[cb][layer] = selected[cb]
			// Ensure monotonic
			if layer > 0 && alloc.CodeBlockPasses[cb][layer] < alloc.CodeBlockPasses[cb][layer-1] {
				alloc.CodeBlockPasses[cb][layer] = alloc.CodeBlockPasses[cb][layer-1]
			}
		}
	}

	return alloc
}

func getPassBytes(passes []t1.PassData, count int) int {
	if count <= 0 {
		return 0
	}
	if count > len(passes) {
		count = len(passes)
	}
	b := passes[count-1].ActualBytes
	if b == 0 {
		b = passes[count-1].Rate
	}
	return b
}

// GetPassesForLayer returns the number of passes to include for a code-block in a specific layer
func (la *LayerAllocation) GetPassesForLayer(codeBlockIndex, layer int) int {
	if codeBlockIndex >= len(la.CodeBlockPasses) {
		return 0
	}
	if layer >= len(la.CodeBlockPasses[codeBlockIndex]) {
		return 0
	}
	return la.CodeBlockPasses[codeBlockIndex][layer]
}

// GetNewPassesForLayer returns the number of NEW passes added in this layer
// (i.e., passes not included in previous layer)
func (la *LayerAllocation) GetNewPassesForLayer(codeBlockIndex, layer int) int {
	if codeBlockIndex >= len(la.CodeBlockPasses) {
		return 0
	}
	if layer >= len(la.CodeBlockPasses[codeBlockIndex]) {
		return 0
	}

	currentPasses := la.CodeBlockPasses[codeBlockIndex][layer]
	if layer == 0 {
		return currentPasses
	}

	previousPasses := la.CodeBlockPasses[codeBlockIndex][layer-1]
	return currentPasses - previousPasses
}
