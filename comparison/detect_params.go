// Raw pixel parameter detection helper
// Suggests possible image parameters based on file size
// Usage: go run detect_params.go <input.bin>

package main

import (
	"fmt"
	"math"
	"os"
	"sort"
)

type ImageParams struct {
	Width      int
	Height     int
	Components int
	BitDepth   int
	Size       int
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input.bin>\n", os.Args[0])
		os.Exit(1)
	}

	inputFile := os.Args[1]

	// Get file size
	info, err := os.Stat(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read file: %v\n", err)
		os.Exit(1)
	}

	fileSize := int(info.Size())

	fmt.Println("Raw Pixel Parameter Detection")
	fmt.Println("=" + repeatString("=", 60))
	fmt.Printf("File: %s\n", inputFile)
	fmt.Printf("Size: %d bytes (%.2f KB, %.2f MB)\n\n",
		fileSize, float64(fileSize)/1024, float64(fileSize)/(1024*1024))

	// Generate possible configurations
	fmt.Println("Possible Image Configurations:")
	fmt.Println("-" + repeatString("-", 60))
	fmt.Printf("%-8s %-8s %-10s %-10s %-12s %s\n",
		"Width", "Height", "Components", "BitDepth", "Size", "Match")
	fmt.Println(repeatString("-", 62))

	candidates := []ImageParams{}

	// Common image dimensions
	dimensions := []int{64, 128, 256, 384, 512, 683, 768, 1024, 1280, 1536, 1920, 2048}
	components := []int{1, 3, 4} // grayscale, RGB, RGBA
	bitDepths := []int{8, 12, 16}

	for _, width := range dimensions {
		for _, height := range dimensions {
			for _, comp := range components {
				for _, bits := range bitDepths {
					bytesPerSample := 1
					if bits > 8 {
						bytesPerSample = 2
					}

					expectedSize := width * height * comp * bytesPerSample
					match := ""

					if expectedSize == fileSize {
						match = "✓ EXACT"
						candidates = append(candidates, ImageParams{
							Width:      width,
							Height:     height,
							Components: comp,
							BitDepth:   bits,
							Size:       expectedSize,
						})
					} else if abs(expectedSize-fileSize) < fileSize/100 { // Within 1%
						match = "~ Close"
						candidates = append(candidates, ImageParams{
							Width:      width,
							Height:     height,
							Components: comp,
							BitDepth:   bits,
							Size:       expectedSize,
						})
					}

					if match != "" {
						compName := getComponentName(comp)
						fmt.Printf("%-8d %-8d %-10s %-10d %-12d %s\n",
							width, height, compName, bits, expectedSize, match)
					}
				}
			}
		}
	}

	// Additional analysis for non-standard sizes
	if len(candidates) == 0 {
		fmt.Println("\nNo standard configurations found. Analyzing factors...")
		analyzeFactors(fileSize)
	} else {
		// Sort by size difference
		sort.Slice(candidates, func(i, j int) bool {
			return abs(candidates[i].Size-fileSize) < abs(candidates[j].Size-fileSize)
		})

		fmt.Println("\n" + repeatString("=", 62))
		fmt.Println("Recommended Configurations:")
		fmt.Println(repeatString("-", 62))

		shown := 0
		for _, c := range candidates {
			if shown >= 5 {
				break
			}
			if c.Size == fileSize {
				compName := getComponentName(c.Components)
				fmt.Printf("  %dx%d, %s, %d-bit\n",
					c.Width, c.Height, compName, c.BitDepth)
				fmt.Printf("    Command: ... %d %d %d %d 0\n\n",
					c.Width, c.Height, c.Components, c.BitDepth)
				shown++
			}
		}

		if shown == 0 {
			fmt.Println("  (No exact matches found)")
		}
	}

	// Factorization hints
	fmt.Println("\n" + repeatString("=", 62))
	fmt.Println("Custom Size Calculation:")
	fmt.Println(repeatString("-", 62))
	fmt.Printf("File size: %d bytes\n", fileSize)
	fmt.Println("\nFor custom dimensions, solve:")
	fmt.Println("  width × height × components × bytes_per_sample = file_size")
	fmt.Println("\nWhere:")
	fmt.Println("  - components: 1 (grayscale), 3 (RGB), 4 (RGBA)")
	fmt.Println("  - bytes_per_sample: 1 (8-bit), 2 (>8-bit)")
	fmt.Println()
}

func analyzeFactors(size int) {
	fmt.Println("\nPrime factorization:")
	factors := primeFactorization(size)
	fmt.Printf("  %d = ", size)
	for i, f := range factors {
		if i > 0 {
			fmt.Print(" × ")
		}
		fmt.Printf("%d", f)
	}
	fmt.Println()

	// Try to suggest dimensions
	fmt.Println("\nPossible dimension factors:")
	divisors := getDivisors(size)
	for _, d := range divisors {
		if d > 32 && d < 4096 {
			other := size / d
			if other > 32 && other < 4096 {
				// Check if it could be a valid image
				for _, comp := range []int{1, 3, 4} {
					if size%(d*comp) == 0 {
						pixels := size / comp
						for _, bytes := range []int{1, 2} {
							if pixels%bytes == 0 {
								actualPixels := pixels / bytes
								if actualPixels == d*other {
									bits := bytes * 8
									if bits <= 8 || bits == 16 {
										fmt.Printf("  %dx%d, %s, %d-bit\n",
											d, other, getComponentName(comp), bits)
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

func primeFactorization(n int) []int {
	factors := []int{}
	for n%2 == 0 {
		factors = append(factors, 2)
		n /= 2
	}
	for i := 3; i <= int(math.Sqrt(float64(n))); i += 2 {
		for n%i == 0 {
			factors = append(factors, i)
			n /= i
		}
	}
	if n > 2 {
		factors = append(factors, n)
	}
	return factors
}

func getDivisors(n int) []int {
	divisors := []int{}
	for i := 1; i <= int(math.Sqrt(float64(n))); i++ {
		if n%i == 0 {
			divisors = append(divisors, i)
			if i != n/i {
				divisors = append(divisors, n/i)
			}
		}
	}
	sort.Ints(divisors)
	return divisors
}

func getComponentName(components int) string {
	switch components {
	case 1:
		return "Grayscale"
	case 3:
		return "RGB"
	case 4:
		return "RGBA"
	default:
		return fmt.Sprintf("%d-comp", components)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
