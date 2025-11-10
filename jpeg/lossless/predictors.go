package lossless

// JPEG Lossless defines 7 predictors for lossless compression
// Ra = left pixel, Rb = above pixel, Rc = above-left pixel

// Predictor applies prediction to a pixel based on its neighbors
// predictor: 1-7 (predictor selection value)
// ra: left pixel value
// rb: above pixel value
// rc: above-left pixel value
// Returns: predicted value
func Predictor(predictor int, ra, rb, rc int) int {
	switch predictor {
	case 1:
		// Predictor 1: Px = Ra
		// Uses left pixel only
		return ra

	case 2:
		// Predictor 2: Px = Rb
		// Uses above pixel only
		return rb

	case 3:
		// Predictor 3: Px = Rc
		// Uses above-left pixel only
		return rc

	case 4:
		// Predictor 4: Px = Ra + Rb - Rc
		// Linear combination
		return ra + rb - rc

	case 5:
		// Predictor 5: Px = Ra + ((Rb - Rc) >> 1)
		// Ra plus half the vertical gradient
		return ra + ((rb - rc) >> 1)

	case 6:
		// Predictor 6: Px = Rb + ((Ra - Rc) >> 1)
		// Rb plus half the horizontal gradient
		return rb + ((ra - rc) >> 1)

	case 7:
		// Predictor 7: Px = (Ra + Rb) / 2
		// Average of left and above
		return (ra + rb) >> 1

	default:
		// Default to predictor 1
		return ra
	}
}

// PredictorName returns the human-readable name for a predictor
func PredictorName(predictor int) string {
	switch predictor {
	case 1:
		return "Left (Ra)"
	case 2:
		return "Above (Rb)"
	case 3:
		return "Above-Left (Rc)"
	case 4:
		return "Ra + Rb - Rc"
	case 5:
		return "Ra + ((Rb - Rc) >> 1)"
	case 6:
		return "Rb + ((Ra - Rc) >> 1)"
	case 7:
		return "(Ra + Rb) / 2"
	default:
		return "Unknown"
	}
}

// SelectBestPredictor analyzes the image and selects the best predictor
// This is a helper function for automatic predictor selection
// Returns the predictor number (1-7) that gives the best compression
func SelectBestPredictor(samples [][]int, width, height int) int {
	// Try each predictor and calculate the entropy/variance
	bestPredictor := 1
	minVariance := int64(1 << 62)

	for p := 1; p <= 7; p++ {
		variance := calculatePredictionVariance(samples, width, height, p)
		if variance < minVariance {
			minVariance = variance
			bestPredictor = p
		}
	}

	return bestPredictor
}

// calculatePredictionVariance calculates the variance of prediction errors
// Lower variance generally means better compression
func calculatePredictionVariance(samples [][]int, width, height, predictor int) int64 {
	var sumSquares int64
	count := 0

	for _, componentSamples := range samples {
		for row := 0; row < height; row++ {
			for col := 0; col < width; col++ {
				sample := componentSamples[row*width+col]

				// Get neighbor values
				var ra, rb, rc int
				if col > 0 {
					ra = componentSamples[row*width+(col-1)]
				}
				if row > 0 {
					rb = componentSamples[(row-1)*width+col]
				}
				if row > 0 && col > 0 {
					rc = componentSamples[(row-1)*width+(col-1)]
				}

				predicted := Predictor(predictor, ra, rb, rc)
				diff := sample - predicted
				sumSquares += int64(diff * diff)
				count++
			}
		}
	}

	if count == 0 {
		return 0
	}

	return sumSquares / int64(count)
}
