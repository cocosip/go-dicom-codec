#!/bin/bash
# JPEG2000 Encoder Comparison Script (Linux/Mac)
# Usage: run_comparison.sh <input.bin> <width> <height> <components> <bitdepth> [signed]

set -e

if [ $# -lt 5 ]; then
    echo "Usage: $0 <input.bin> <width> <height> <components> <bitdepth> [signed]"
    echo "Example: $0 /path/to/1_raw_pixels.bin 512 512 3 8 0"
    exit 1
fi

INPUT_FILE="$1"
WIDTH="$2"
HEIGHT="$3"
COMPONENTS="$4"
BITDEPTH="$5"
SIGNED="${6:-0}"

echo "============================================================"
echo "JPEG2000 Encoder Comparison"
echo "============================================================"
echo ""
echo "Input file: $INPUT_FILE"
echo "Dimensions: ${WIDTH}x${HEIGHT}"
echo "Components: $COMPONENTS"
echo "Bit depth: $BITDEPTH"
echo "Signed: $SIGNED"
echo ""

# Check if input file exists
if [ ! -f "$INPUT_FILE" ]; then
    echo "Error: Input file not found: $INPUT_FILE"
    exit 1
fi

# Step 1: Build OpenJPEG encoder if needed
echo "[1/5] Building OpenJPEG encoder..."
if [ ! -f "cpp/openjpeg_encoder" ]; then
    cd cpp
    g++ -o openjpeg_encoder openjpeg_encoder.cpp \
        -I../../fo-dicom-codec-code/Native/Common/OpenJPEG \
        ../../fo-dicom-codec-code/Native/Common/OpenJPEG/*.c \
        -lm -lpthread
    if [ $? -ne 0 ]; then
        echo "Error: Failed to build OpenJPEG encoder"
        echo "Please ensure you have g++ installed and OpenJPEG source files are available"
        exit 1
    fi
    cd ..
    echo "  Built successfully!"
else
    echo "  Already built."
fi
echo ""

# Step 2: Encode with OpenJPEG
echo "[2/5] Encoding with OpenJPEG..."
cpp/openjpeg_encoder "$INPUT_FILE" output_openjpeg.j2k $WIDTH $HEIGHT $COMPONENTS $BITDEPTH $SIGNED
echo ""

# Step 3: Encode with Go
echo "[3/5] Encoding with Go..."
go run go_encoder.go "$INPUT_FILE" output_go.j2k $WIDTH $HEIGHT $COMPONENTS $BITDEPTH $SIGNED
echo ""

# Step 4: Basic comparison
echo "[4/5] Comparing results (basic)..."
go run compare_j2k.go output_openjpeg.j2k output_go.j2k
echo ""

# Step 5: Detailed comparison
echo "[5/5] Comparing results (detailed)..."
go run compare_j2k.go output_openjpeg.j2k output_go.j2k detailed > comparison_detailed.txt
echo "  Detailed comparison saved to: comparison_detailed.txt"
echo ""

echo "============================================================"
echo "Comparison completed!"
echo "============================================================"
echo ""
echo "Output files:"
echo "  - output_openjpeg.j2k (OpenJPEG encoded)"
echo "  - output_go.j2k (Go encoded)"
echo "  - comparison_detailed.txt (detailed analysis)"
echo ""
